"""Offline reachability model; does not claim kube-router packet enforcement."""
import ipaddress
import json
import os
from pathlib import Path
import subprocess
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT.parents[1]
NAME = 'app.kubernetes.io/name'
INSTANCE = 'app.kubernetes.io/instance'
NS_NAME = 'kubernetes.io/metadata.name'
PROM = {NAME: 'prometheus', 'prometheus': 'prometheus-kube-prometheus-prometheus'}
TRAEFIK = {NAME: 'traefik', INSTANCE: 'traefik-kube-system'}


def pod(namespace, labels):
    return {'namespace': namespace, 'labels': labels, 'ip': '10.42.250.2'}


def matches(selector, labels):
    # Fail rather than silently mis-model a future selector operator.
    if set(selector) - {'matchLabels'}:
        raise ValueError(f'Unsupported selector: {selector}')
    return all(labels.get(k) == v for k, v in selector.get('matchLabels', {}).items())


def peer_matches(peer, source, destination):
    if 'ipBlock' in peer:
        address = ipaddress.ip_address(source['ip'])
        block = peer['ipBlock']
        return address in ipaddress.ip_network(block['cidr']) and not any(
            address in ipaddress.ip_network(cidr) for cidr in block.get('except', [])
        )
    if 'namespace' not in source:
        return False
    if 'namespaceSelector' in peer:
        if not matches(peer['namespaceSelector'], {NS_NAME: source['namespace']}):
            return False
    elif source['namespace'] != destination['namespace']:
        return False
    return matches(peer.get('podSelector', {}), source.get('labels', {}))


def allowed(policies, source, destination, port, protocol='TCP'):
    selected = [p for p in policies if p['metadata']['namespace'] == destination['namespace']
                and matches(p['spec']['podSelector'], destination['labels'])
                and 'Ingress' in p['spec']['policyTypes']]
    if not selected:
        return True
    for policy in selected:
        for rule in policy['spec'].get('ingress', []):
            ports = rule.get('ports', [])
            if ports and not any(p['port'] == port and p.get('protocol', 'TCP') == protocol
                                 for p in ports):
                continue
            peers = rule.get('from', [])
            if not peers or any(peer_matches(p, source, destination) for p in peers):
                return True
    return False


class Policies(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inventory = json.loads((ROOT / 'tests/inventory.json').read_text())
        cls.prepared = [p for path in sorted((ROOT / 'namespaces').glob('*/policies.yaml'))
                        for p in yaml.safe_load_all(path.read_text())]
        cls.deployed_monitoring = list(yaml.safe_load_all(
            (REPO / 'f3s/prometheus/manifests/prometheus-networkpolicy.yaml').read_text()))
        cls.policies = cls.prepared + cls.deployed_monitoring
        cls.prometheus = pod('monitoring', PROM)
        cls.traefik = pod('kube-system', TRAEFIK)
        cls.unrelated = pod('services', {'app': 'unrelated'})

    def assert_path(self, source, namespace, labels, port, expected, protocol='TCP'):
        self.assertEqual(allowed(self.policies, source, pod(namespace, labels), port, protocol),
                         expected, (source, namespace, labels, port, protocol))

    def test_every_observed_namespace_has_default_deny_ingress_only(self):
        expected = set(self.inventory['namespaces'])
        self.assertEqual({p['metadata']['namespace'] for p in self.prepared}, expected)
        for namespace in expected:
            defaults = [p for p in self.prepared if p['metadata']['namespace'] == namespace
                        and p['spec']['podSelector'] == {}]
            self.assertEqual(len(defaults), 1, namespace)
            self.assertEqual(defaults[0]['spec']['ingress'], [])
            self.assert_path(self.unrelated, namespace, {'app': 'future-workload'}, 5432, False)
        for policy in self.policies:
            self.assertEqual(policy['spec']['policyTypes'], ['Ingress'])
            self.assertNotIn('egress', policy['spec'])

    def test_every_allow_has_destinations_sources_and_ports(self):
        for policy in self.prepared:
            for rule in policy['spec']['ingress']:
                self.assertTrue(policy['spec']['podSelector'].get('matchLabels'), policy)
                self.assertTrue(rule['from'], policy)
                self.assertTrue(rule['ports'], policy)
                for peer in rule['from']:
                    if peer == {'namespaceSelector': {}}:
                        self.assertEqual(policy['metadata']['name'], 'dns')
                    elif 'ipBlock' not in peer:
                        self.assertTrue(peer['podSelector']['matchLabels'], policy)

    def test_first_selecting_write_includes_every_receiver_flow(self):
        sources = [pod(row['namespace'], row['labels']) for row in self.inventory['pods']
                   if not row['hostNetwork']] + [self.unrelated]
        sources += [{'ip': peer['ipBlock']['cidr'].split('/')[0]}
                    for policy in self.prepared for rule in policy['spec']['ingress']
                    for peer in rule['from'] if 'ipBlock' in peer]
        # Check each actual prefix, not just the eventual namespace default deny.
        prefix = list(self.deployed_monitoring)
        receivers = set()
        for policy in self.prepared:
            prefix.append(policy)
            selector = policy['spec']['podSelector']
            if not selector:
                continue
            namespace = policy['metadata']['namespace']
            key = (namespace, json.dumps(selector, sort_keys=True))
            self.assertNotIn(key, receivers, 'Split receiver policies cause a known transient denial')
            receivers.add(key)
            destination = pod(namespace, selector['matchLabels'])
            ports = {p['port'] for rule in policy['spec']['ingress'] for p in rule['ports']} | {65535}
            for source in sources:
                for port in ports:
                    for protocol in ['TCP', 'UDP']:
                        self.assertEqual(allowed(prefix, source, destination, port, protocol),
                                         allowed(self.policies, source, destination, port, protocol),
                                         (policy['metadata'], source, port, protocol))

    def test_actual_ingress_backends_allow_only_traefik_pods(self):
        for route in self.inventory['ingressBackends']:
            for source, expected in [(self.traefik, True), (self.unrelated, False),
                                     (pod('services', TRAEFIK), False)]:
                with self.subTest(service=route['service'], source=source):
                    self.assert_path(source, route['namespace'], route['selector'],
                                     route['port'], expected)

    def test_each_app_reaches_its_own_database_and_immich_dependencies(self):
        immich = {INSTANCE: 'immich', NAME: 'server', 'app.kubernetes.io/controller': 'main'}
        cases = [({'app': 'miniflux-server'}, {'app': 'miniflux-postgres'}, 5432),
                 (immich, {'app': 'immich-postgres'}, 5432),
                 (immich, {INSTANCE: 'immich', NAME: 'valkey',
                           'app.kubernetes.io/controller': 'main'}, 6379),
                 (immich, {INSTANCE: 'immich', NAME: 'machine-learning',
                           'app.kubernetes.io/controller': 'main'}, 3003)]
        for source, destination, port in cases:
            for namespace, expected in [('services', True), ('test', False)]:
                self.assert_path(pod(namespace, source), 'services', destination, port, expected)
            for other in [self.unrelated, self.traefik, self.prometheus]:
                self.assert_path(other, 'services', destination, port, False)
            self.assert_path(pod('services', source), 'services', destination, port + 1, False)
        self.assert_path(pod('services', {'app': 'miniflux-server'}), 'services',
                         {'app': 'immich-postgres'}, 5432, False)
        self.assert_path(pod('services', immich), 'services', {'app': 'miniflux-postgres'}, 5432, False)

    def test_metrics_and_alert_delivery_are_scoped(self):
        cases = [('services', {'app': 'goprecords'}, [8080]),
                 ('monitoring', {'app': 'pushgateway'}, [9091]),
                 ('monitoring', {NAME: 'kube-state-metrics', INSTANCE: 'prometheus'}, [8080]),
                 ('monitoring', {'app': 'kube-prometheus-stack-operator', 'release': 'prometheus'}, [10250]),
                 ('monitoring', {NAME: 'alertmanager', 'alertmanager': 'prometheus-kube-prometheus-alertmanager'}, [9093, 8080]),
                 ('kube-system', {'k8s-app': 'kube-dns'}, [9153]),
                 ('kube-system', TRAEFIK, [9100])]
        cases += [('cicd', {NAME: 'argocd-' + app, INSTANCE: 'argocd'}, [port])
                  for app, port in [('application-controller', 8082), ('repo-server', 8084), ('server', 8083)]]
        cases += [('cert-manager', {'app': app, INSTANCE: 'cert-manager'}, [9402])
                  for app in ['cert-manager', 'cainjector', 'webhook']]
        for namespace, destination, ports in cases:
            for port in ports:
                self.assert_path(self.prometheus, namespace, destination, port, True)
                self.assert_path(self.unrelated, namespace, destination, port, False)
                self.assert_path(pod('services', PROM), namespace, destination, port, False)
                self.assert_path(self.prometheus, namespace, destination, port, False, 'UDP')

    def test_monitoring_additive_union_preserves_deployed_receiver_boundary(self):
        proxy = {NAME: 'prometheus-read-proxy'}
        for row in self.inventory['pods']:
            if row['hostNetwork']:
                continue  # Node/host-network exceptions require runtime/host controls.
            source = pod(row['namespace'], row['labels'])
            expected = row['namespace'] == 'monitoring' and (
                matches({'matchLabels': PROM}, row['labels']) or
                matches({'matchLabels': proxy}, row['labels']))
            self.assert_path(source, 'monitoring', PROM, 9090, expected)
            self.assert_path(source, 'monitoring', proxy, 8080, False)
        for address in ['192.168.2.110', '192.168.2.111', '192.168.2.200', '192.168.2.201']:
            self.assert_path({'ip': address}, 'monitoring', proxy, 8080, True)
            self.assert_path({'ip': address}, 'monitoring', PROM, 9090, False)
        for address in ['192.168.1.6', '192.168.2.130', '192.168.2.202']:
            self.assert_path({'ip': address}, 'monitoring', proxy, 8080, False)
        self.assert_path(pod('services', proxy), 'monitoring', PROM, 9090, False)
        self.assert_path(self.prometheus, 'monitoring', PROM, 8080, True)
        self.assert_path(pod('monitoring', proxy), 'monitoring', PROM, 8080, False)

    def test_gitops_dependencies_and_cache_separation(self):
        argo = lambda name: {NAME: 'argocd-' + name, INSTANCE: 'argocd'}
        for name in ['server', 'repo-server', 'application-controller']:
            self.assert_path(pod('cicd', argo(name)), 'cicd', argo('redis'), 6379, True)
        for name in ['server', 'application-controller', 'applicationset-controller']:
            self.assert_path(pod('cicd', argo(name)), 'cicd', argo('repo-server'), 8081, True)
        self.assert_path(pod('cicd', argo('repo-server')), 'services', {'app': 'forgejo'}, 3000, True)
        self.assert_path(pod('services', argo('repo-server')), 'services', {'app': 'forgejo'}, 3000, False)
        self.assert_path(self.unrelated, 'cicd', argo('redis'), 6379, False)
        self.assert_path(self.unrelated, 'cicd', argo('server'), 8080, False)

    def test_dns_and_control_plane_webhooks_have_explicit_ports(self):
        for namespace in self.inventory['namespaces']:
            for protocol in ['TCP', 'UDP']:
                self.assert_path(pod(namespace, {'app': 'future-workload'}), 'kube-system',
                                 {'k8s-app': 'kube-dns'}, 53, True, protocol)
        for namespace, labels in [('kube-system', {'k8s-app': 'metrics-server'}),
                                  ('cert-manager', {'app': 'webhook', INSTANCE: 'cert-manager'}),
                                  ('monitoring', {'app': 'kube-prometheus-stack-operator', 'release': 'prometheus'})]:
            for address in ['192.168.2.120', '192.168.2.121', '192.168.2.122', '10.42.0.1', '10.42.1.0', '10.42.2.0']:
                self.assert_path({'ip': address}, namespace, labels, 10250, True)
            self.assert_path(self.unrelated, namespace, labels, 10250, False)

    def test_nodeport_destinations_do_not_allow_ordinary_pods(self):
        for namespace, labels, ports in [('infra', {'app': 'docker-registry'}, [5000]),
                                          ('services', {'app': 'protonbridge'}, [25, 143]),
                                          ('services', {'app': 'forgejo'}, [2222]),
                                          ('services', {'app': 'anki-sync-server'}, [8080])]:
            for port in ports:
                for address in ['192.168.2.200', '192.168.2.201']:
                    self.assert_path({'ip': address}, namespace, labels, port, True)
                self.assert_path(self.unrelated, namespace, labels, port, False)
                self.assert_path({'ip': '192.168.2.202'}, namespace, labels, port, False)
                # After Cluster SNAT, original external identities cannot be distinguished.
                self.assert_path({'ip': '10.42.1.0'}, namespace, labels, port, True)

    def test_laptop_wg2_reaches_only_the_syncthing_sync_port(self):
        # n44: the Local NodePort syncthing-lan keeps the laptop's source, so
        # the receiver admits exactly 10.99.0.2 on TCP 22000 and nothing else.
        syncthing = {'app': 'syncthing'}
        self.assert_path({'ip': '10.99.0.2'}, 'services', syncthing, 22000, True)
        self.assert_path({'ip': '10.99.0.2'}, 'services', syncthing, 22000, False, 'UDP')
        self.assert_path({'ip': '10.99.0.2'}, 'services', syncthing, 8384, False)
        # The phone, other wg2/LAN hosts, ordinary pods and rewritten node/CNI
        # sources (Cluster SNAT) get no sync-port access.
        for source in [{'ip': '10.99.0.3'}, {'ip': '10.99.0.4'}, {'ip': '10.20.0.130'},
                       {'ip': '192.168.2.200'}, {'ip': '10.42.0.0'}, {'ip': '10.42.1.1'},
                       {'ip': '10.20.0.120'}, self.unrelated, self.prometheus]:
            self.assert_path(source, 'services', syncthing, 22000, False)
        # Traefik keeps the web GUI, but not the sync port.
        self.assert_path(self.traefik, 'services', syncthing, 8384, True)
        self.assert_path(self.traefik, 'services', syncthing, 22000, False)
        # The laptop gains nothing elsewhere in the namespace.
        for database in ['immich-postgres', 'miniflux-postgres']:
            self.assert_path({'ip': '10.99.0.2'}, 'services', {'app': database}, 5432, False)

    def test_controller_pis_push_only_to_the_pushgateway(self):
        # s44: the Local NodePort pushgateway-lan keeps the Pis' lab sources,
        # so the receiver admits exactly 10.20.0.125/.126 on TCP 9091.
        pushgateway = {'app': 'pushgateway'}
        for address in ['10.20.0.125', '10.20.0.126']:
            self.assert_path({'ip': address}, 'monitoring', pushgateway, 9091, True)
            self.assert_path({'ip': address}, 'monitoring', pushgateway, 9091, False, 'UDP')
            self.assert_path({'ip': address}, 'monitoring', pushgateway, 9090, False)
            # Nothing else in monitoring: not Prometheus, its read proxy or
            # Alertmanager.
            self.assert_path({'ip': address}, 'monitoring', PROM, 9090, False)
            self.assert_path({'ip': address}, 'monitoring',
                             {NAME: 'prometheus-read-proxy'}, 8080, False)
            self.assert_path({'ip': address}, 'monitoring',
                             {NAME: 'alertmanager', 'alertmanager': 'prometheus-kube-prometheus-alertmanager'},
                             9093, False)
            self.assert_path({'ip': address}, 'services', {'app': 'miniflux-postgres'}, 5432, False)
        # The Pis' old LAN addresses, the other Pis, the r-nodes and storage
        # hosts, rewritten node/CNI sources, wg2 clients and ordinary pods
        # get no write access.
        for source in [{'ip': '192.168.1.125'}, {'ip': '192.168.1.126'}, {'ip': '10.20.0.127'},
                       {'ip': '10.20.0.128'}, {'ip': '10.20.0.120'}, {'ip': '10.20.0.130'},
                       {'ip': '10.42.0.0'}, {'ip': '10.42.2.1'}, {'ip': '10.99.0.2'},
                       {'ip': '192.168.2.200'}, self.unrelated, pod('monitoring', {'app': 'other'})]:
            self.assert_path(source, 'monitoring', pushgateway, 9091, False)
        # The Prometheus scrape stays.
        self.assert_path(self.prometheus, 'monitoring', pushgateway, 9091, True)

    def test_laptop_and_phone_reach_web_without_database_or_receiver_access(self):
        servicelb = {'svccontroller.k3s.cattle.io/svcname': 'traefik',
                     'svccontroller.k3s.cattle.io/svcnamespace': 'kube-system'}
        for address, permitted in [('192.168.2.200', True), ('192.168.2.201', True),
                                   ('192.168.2.202', False)]:
            for labels, ports in [(TRAEFIK, [8000, 8443]), (servicelb, [80, 443])]:
                for port in ports:
                    self.assert_path({'ip': address}, 'kube-system', labels, port, permitted)
                self.assert_path({'ip': address}, 'kube-system', labels, 9100, False)
            for database in ['immich-postgres', 'miniflux-postgres']:
                self.assert_path({'ip': address}, 'services', {'app': database}, 5432, False)
            self.assert_path({'ip': address}, 'monitoring', PROM, 9090, False)

    def test_migrated_storage_relays_reach_only_existing_web_receivers(self):
        receivers = [(TRAEFIK, (8000, 8443)),
                     ({'svccontroller.k3s.cattle.io/svcname': 'traefik',
                       'svccontroller.k3s.cattle.io/svcnamespace': 'kube-system'}, (80, 443))]
        for address in ['10.20.0.130', '10.20.0.131', '10.20.0.138']:
            for labels, ports in receivers:
                for port in ports:
                    self.assert_path({'ip': address}, 'kube-system', labels, port, True)
                    self.assert_path({'ip': address}, 'kube-system', labels, port, False, 'UDP')
                self.assert_path({'ip': address}, 'kube-system', labels, 9100, False)
            self.assert_path({'ip': address}, 'services', {'app': 'miniflux-postgres'}, 5432, False)
        # Neither adjacent lab hosts, direct wg2 clients nor the relays' old
        # LAN addresses (retired in task i24, removed in task 174) inherit
        # relay trust.
        for address in ['10.20.0.129', '10.20.0.132', '10.20.0.133',
                        '10.20.0.139', '10.99.0.2', '10.99.0.3',
                        '192.168.1.130', '192.168.1.131', '192.168.1.138']:
            for labels, ports in receivers:
                for port in ports:
                    self.assert_path({'ip': address}, 'kube-system', labels, port, False)

    def test_node_paths_use_lab_addresses_not_retired_old_lan(self):
        # Task 174: the r-nodes' old LAN /32s (192.168.1.120-122) were
        # removed after task i24 retired them. Every receiver that admitted
        # them keeps its lab (10.20.0.x), wg0 and CNI node paths; the old
        # addresses, and any other old-LAN host, are now denied.
        receivers = [('kube-system', {'k8s-app': 'kube-dns'}, 53),
                     ('kube-system', {'k8s-app': 'metrics-server'}, 10250),
                     ('cert-manager', {'app': 'webhook', INSTANCE: 'cert-manager'}, 10250),
                     ('monitoring', {'app': 'kube-prometheus-stack-operator', 'release': 'prometheus'}, 10250),
                     ('kube-system', TRAEFIK, 8000),
                     ('infra', {'app': 'docker-registry'}, 5000),
                     ('services', {'app': 'protonbridge'}, 143),
                     ('services', {'app': 'forgejo'}, 2222),
                     ('services', {'app': 'anki-sync-server'}, 8080)]
        for namespace, labels, port in receivers:
            for address in ['10.20.0.120', '10.20.0.121', '10.20.0.122', '192.168.2.120']:
                self.assert_path({'ip': address}, namespace, labels, port, True)
            for address in ['192.168.1.120', '192.168.1.121', '192.168.1.122', '192.168.1.1']:
                self.assert_path({'ip': address}, namespace, labels, port, False)
        # No policy source keeps any address of the retired old LAN.
        old_lan = [peer['ipBlock']['cidr'] for policy in self.prepared
                   for rule in (policy['spec'].get('ingress') or [])
                   for peer in rule.get('from', [])
                   if 'ipBlock' in peer and peer['ipBlock']['cidr'].startswith('192.168.1.')]
        self.assertEqual(old_lan, [])

    def test_cross_node_nodeport_snat_is_a_documented_receiver_bypass(self):
        # Observed KUBE-EXT/MARK-MASQ/POSTROUTING rules rewrite a cross-node
        # NodePort path before the remote receiver's source-ipset match.
        # This models that rewrite explicitly; it is not a packet test.
        rewritten = {'ip': '10.42.0.0'}
        for namespace, labels, port in [('infra', {'app': 'docker-registry'}, 5000),
                                        ('services', {'app': 'protonbridge'}, 143),
                                        ('services', {'app': 'forgejo'}, 2222),
                                        ('services', {'app': 'anki-sync-server'}, 8080)]:
            self.assert_path(self.unrelated, namespace, labels, port, False)
            self.assert_path(rewritten, namespace, labels, port, True)
        for database in ['immich-postgres', 'miniflux-postgres']:
            self.assert_path(self.unrelated, 'services', {'app': database}, 5432, False)
            self.assert_path(rewritten, 'services', {'app': database}, 5432, False)

    def test_applications_are_disabled_separate_and_manual(self):
        paths = sorted((REPO / 'f3s/argocd-apps/network-policies').glob('*'))
        self.assertEqual(len(paths), len(self.inventory['namespaces']))
        for path in paths:
            self.assertTrue(path.name.endswith('.yaml.disabled'))
            app = yaml.safe_load(path.read_text())
            self.assertNotIn('automated', app['spec']['syncPolicy'])
            self.assertNotIn('finalizers', app['metadata'])
            namespace = app['spec']['destination']['namespace']
            self.assertEqual(app['spec']['source']['path'], 'f3s/network-policies/namespaces/' + namespace)

    def test_cicd_chart_render_removes_all_additive_chart_policies(self):
        chart = os.environ.get('ARGOCD_CHART')
        if not chart:
            self.skipTest('Set ARGOCD_CHART to the local argo-cd-10.10.5 chart archive')
        command = ['helm', 'template', 'argocd', chart, '--namespace', 'cicd',
                   '-f', str(REPO / 'f3s/argocd/values.yaml')]
        baseline = list(yaml.safe_load_all(subprocess.check_output(command, text=True)))
        self.assertTrue(any(p and p['kind'] == 'NetworkPolicy' and p['spec'].get('ingress') == [{}]
                            for p in baseline))
        chart_names = {p['metadata']['name'] for p in baseline
                       if p and p['kind'] == 'NetworkPolicy'}
        replacement_names = {p['metadata']['name'] for p in self.prepared
                             if p['metadata']['namespace'] == 'cicd'}
        self.assertFalse(chart_names & replacement_names,
                         'Helm removal must not delete same-named replacement policies')
        rendered = list(yaml.safe_load_all(subprocess.check_output(
            command + ['-f', str(ROOT / 'cicd-chart-values.yaml')], text=True)))
        self.assertFalse(any(p and p['kind'] == 'NetworkPolicy' for p in rendered))

    def test_allow_waves_precede_default_deny_and_cicd_activation_patch_is_applicable(self):
        for path in (ROOT / 'namespaces').glob('*/policies.yaml'):
            documents = list(yaml.safe_load_all(path.read_text()))
            self.assertEqual(documents[-1]['metadata']['name'], 'default-deny-ingress')
            self.assertEqual(documents[-1]['metadata']['annotations']['argocd.argoproj.io/sync-wave'], '0')
            for policy in documents[:-1]:
                self.assertEqual(policy['metadata']['annotations']['argocd.argoproj.io/sync-wave'], '-1')
        patch = ROOT / 'cicd-activation.patch'
        check = subprocess.run(['git', 'apply', '--check', str(patch)], cwd=REPO, capture_output=True)
        if check.returncode:
            # The same fixture remains usable after its approved activation.
            subprocess.run(['git', 'apply', '--reverse', '--check', str(patch)],
                           cwd=REPO, check=True, capture_output=True)
        added = [line for line in patch.read_text().splitlines() if line.startswith('+    helm ')]
        self.assertEqual(len(added), 2)
        self.assertTrue(all('-f ../network-policies/cicd-chart-values.yaml' in line for line in added))

    def test_native_openapi_schema_offline(self):
        path = os.environ.get('NETWORK_POLICY_OPENAPI')
        if not path:
            self.skipTest('Set NETWORK_POLICY_OPENAPI to a saved networking.k8s.io/v1 OpenAPI v3 schema')
        import jsonschema
        document = json.loads(Path(path).read_text())
        schema = {'$ref': '#/components/schemas/io.k8s.api.networking.v1.NetworkPolicy',
                  'components': document['components']}
        validator = jsonschema.Draft4Validator(schema)
        for policy in self.policies:
            validator.validate(policy)
        malformed = json.loads(json.dumps(self.prepared[0]))
        malformed['spec']['policyTypes'] = 'Ingress'
        with self.assertRaises(jsonschema.ValidationError):
            validator.validate(malformed)


if __name__ == '__main__':
    unittest.main()
