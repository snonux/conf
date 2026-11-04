#!/usr/bin/env python3
"""Render inactive additive application allowlists from the current tracked files.

No Kubernetes client, network request, service restart or source rewrite occurs.
Output must be an empty directory outside the tracked manifest trees.
"""
import argparse
import ipaddress
from pathlib import Path
import yaml

ROOT = Path(__file__).resolve().parents[2]
MAP = {f'192.168.1.{n}/32': f'10.20.0.{n}/32'
       for n in (120, 121, 122, 125, 126, 127, 128, 130, 131, 132, 133)}
MAP.update({'192.168.2.200/32': '10.99.0.2/32',
            '192.168.2.201/32': '10.99.0.3/32'})


def extend(node):
    """Preserve selectors/ports and old peers; add only equivalent /32 peers."""
    count = 0
    if isinstance(node, dict):
        for key, value in node.items():
            if key == 'from' and isinstance(value, list):
                for peer in list(value):
                    block = peer.get('ipBlock', {})
                    target = MAP.get(block.get('cidr'))
                    if target and not block.get('except'):
                        assert ipaddress.ip_network(target).prefixlen == 32
                        candidate = {'ipBlock': {'cidr': target}}
                        if candidate not in value:
                            value.append(candidate)
                            count += 1
            else:
                count += extend(value)
    elif isinstance(node, list):
        count += sum(extend(value) for value in node)
    return count


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    output = args.output.resolve()
    protected = [ROOT / 'f3s/network-policies', ROOT / 'f3s/prometheus/manifests']
    if any(output == p or p in output.parents for p in protected):
        parser.error('output must be outside active manifest trees')
    if output.exists() and any(output.iterdir()):
        parser.error('output must be empty')
    sources = sorted((ROOT / 'f3s/network-policies/namespaces').glob('*/policies.yaml'))
    sources.append(ROOT / 'f3s/prometheus/manifests/prometheus-networkpolicy.yaml')
    rendered = []
    for source in sources:
        docs = list(yaml.safe_load_all(source.read_text()))
        additions = extend(docs)
        if additions:
            rendered.append((source, docs, additions))
    output.mkdir(parents=True, exist_ok=True)
    for source, docs, additions in rendered:
        name = str(source.relative_to(ROOT)).replace('/', '--')
        (output / name).write_text('# INACTIVE candidate: source/port acceptance required.\n'
                                   + yaml.safe_dump_all(docs, sort_keys=False))
        print(f'{name}: {additions} exact source additions')


if __name__ == '__main__':
    main()
