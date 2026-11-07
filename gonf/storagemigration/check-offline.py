#!/usr/bin/env python3
"""Exercise renderers and record candidate plans without contacting any host."""
from pathlib import Path
import os
import subprocess
import tempfile

root = Path(__file__).resolve().parents[2]
source = r'''
package main
import (
 "fmt"
 "os"
 "strings"
 "github.com/snonux/conf/gonf/cluster"
 "github.com/snonux/conf/gonf/tasks"
 sm "github.com/snonux/conf/gonf/storagemigration"
 "github.com/snonux/gonf/api"
 "github.com/snonux/gonf/plan"
)
func must(ok bool, message string) { if !ok { panic(message) } }
func main() {
 server, err := os.ReadFile("../f3s/freebsd-hosts/nfs/stunnel.conf"); if err != nil { panic(err) }
 relay, err := os.ReadFile("../f3s/freebsd-hosts/relayd/relayd.conf"); if err != nil { panic(err) }
 if strings.Contains(string(server), "[nfs-tls-compat]") {
  global, _, _ := strings.Cut(string(server), "[nfs-tls]"); _, service, _ := strings.Cut(string(server), "[nfs-tls-compat]")
  server = []byte(global+"[nfs-tls]"+service)
 }
 if strings.Contains(string(relay), "\n\n# Retained until old clients migrate.\n") {
  primary, _, _ := strings.Cut(string(relay), "\n\n# Retained until old clients migrate.\n")
  trailing := len(relay)-len(strings.TrimRight(string(relay), "\n"))
  relay = []byte(strings.ReplaceAll(primary,"10.20.0.","192.168.1.")+strings.Repeat("\n",trailing))
 }
 got, err := sm.Server(string(server)); must(err == nil, "server render")
 must(strings.ReplaceAll(got, sm.LabVIP, sm.OldVIP) == string(server), "server changed unrelated policy")
 compatible, err := sm.CompatibleServer(string(server)); must(err == nil, "compatible server render")
 must(strings.Count(compatible, "accept = "+sm.LabVIP+":2323") == 1 && strings.Count(compatible, "accept = "+sm.OldVIP+":2323") == 1, "compatible server lost one endpoint")
 must(strings.Count(compatible, "verify = 2") == 2 && strings.Count(compatible, "requireCert = yes") == 2, "compatible server changed mutual TLS")
 repeated, err := sm.CompatibleServer(compatible); must(err == nil && repeated == compatible, "compatible server reapply drift")
 _, err = sm.CompatibleServer(compatible+"\n[extra]\naccept = 127.0.0.1:2323\n"); must(err != nil, "compatible server accepted extra service")
 for _, bad := range []string{"", got, string(server)+string(server),
  "accept = "+sm.OldVIP+":23230\n",
  "# accept = "+sm.OldVIP+":2323\naccept = 127.0.0.1:2323\n",
  "; accept = "+sm.OldVIP+":2323\n",
  "accept = 127.0.0.1:2323\n",
  string(server)+"\naccept = 127.0.0.1:2323\n",
  string(server)+"\nAccept = 127.0.0.1:2323\n",
  "accept = "+sm.OldVIP+":2323\naccept = "+sm.OldVIP+":2323\n"} {
  _, err = sm.Server(bad); must(err != nil, "server accepted drift")
 }
 got, err = sm.Relayd(string(relay)); must(err == nil, "relayd render")
 must(strings.ReplaceAll(got, "10.20.0.", "192.168.1.") == string(relay), "relayd changed unrelated config")
 compatible, err = sm.CompatibleRelayd(string(relay)); must(err == nil, "compatible relayd render")
 must(strings.Count(compatible, "listen on "+sm.LabVIP) == 2 && strings.Count(compatible, "listen on "+sm.OldVIP) == 2, "compatible relayd lost listeners")
 must(strings.Count(compatible, "table <k3s_nodes>") == 1 && strings.Contains(compatible, "10.20.0.120 10.20.0.121 10.20.0.122"), "compatible relayd changed backend ownership")
 repeated, err = sm.CompatibleRelayd(compatible); must(err == nil && repeated == compatible, "compatible relayd reapply drift")
 _, err = sm.CompatibleRelayd(compatible+compatible); must(err != nil, "compatible relayd accepted duplicate promotion")
 for _, bad := range []string{"", got, strings.ReplaceAll(string(relay), "192.168.1.121", "192.168.1.120"), strings.ReplaceAll(string(relay), "192.168.1.121", "10.20.0.121"), string(relay)+string(relay)} {
  _, err = sm.Relayd(bad); must(err != nil, "relayd accepted drift, duplicate or mixed subnet")
 }
 for _, identity := range []string{"r0", "r1", "r2", "earth"} {
  original := "cert = /etc/stunnel/"+identity+"-stunnel.pem\nverify = 2\naccept = 127.0.0.1:2323\nconnect = "+sm.OldVIP+":2323\n"
  got, err = sm.Client(original); must(err == nil, "client render")
  must(strings.ReplaceAll(got, sm.LabVIP, sm.OldVIP) == original, "client identity drift")
  _, err = sm.Client(got); must(err != nil, "client accepted already promoted source")
 }
 // Exercise the real public client renderer against complete directive failures.
 for _, bad := range []string{
  "connect = "+sm.OldVIP+":23230\n",
  "# connect = "+sm.OldVIP+":2323\nconnect = 127.0.0.1:2323\n",
  "# connect = "+sm.OldVIP+":2323\n",
  "connect = 127.0.0.1:2323\n",
  "connect = "+sm.OldVIP+":2323\nconnect = "+sm.OldVIP+":2323\n",
  "connect = "+sm.OldVIP+":2323\nconnect = 127.0.0.1:2323\n",
  "connect = "+sm.OldVIP+":2323\nConnect = 127.0.0.1:2323\n"} {
  _, err = sm.Client(bad); must(err != nil, "client accepted malformed active endpoint")
 }
 for _, directive := range []string{"accept", "connect"} {
  original := "# "+directive+" = "+sm.OldVIP+":2323\n  "+directive+" = "+sm.OldVIP+":2323\ncert = existing.pem\nkey = existing-key.pem\n"
  var rendered string
  if directive == "accept" { rendered, err = sm.Server(original) } else { rendered, err = sm.Client(original) }
  must(err == nil, "active endpoint after comment rejected")
  expected := strings.Replace(original, "  "+directive+" = "+sm.OldVIP, "  "+directive+" = "+sm.LabVIP, 1)
  must(rendered == expected, "comment, indentation or credentials path changed")
 }
 for _, bad := range []string{
  strings.Replace(string(relay), "port 80", "port 800", 1),
  strings.Replace(string(relay), "listen on", "# listen on", 1),
  strings.Replace(string(relay), "listen on "+sm.OldVIP, "listen on 127.0.0.1", 1),
  string(relay)+"\nlisten on "+sm.OldVIP+" port 80\n"} {
  _, err = sm.Relayd(bad); must(err != nil, "relayd accepted malformed active listener")
 }
 cluster.Register(); tasks.Register()
 api.Task("offline_storage_probe", "", func() { api.Noop("storage-probe") })
 api.Aggregate("offline_storage_aggregate", "", "(offline_storage_probe|freebsd_(carp|nfs|relayd)_migration_candidate|rnodes_nfs_client_migration_candidate|system_nfs_client_migration_candidate)$")
 excluded, err := api.RecordPlan("excluded", "", "offline_storage_aggregate"); if err != nil { panic(err) }
 for _, op := range excluded { must(op.Op == plan.KindPlan || op.Op == plan.KindWhenBegin || op.Op == plan.KindWhenEnd || op.Op == plan.KindNoop, "operational candidate joined aggregate") }
 for _, task := range []string{"freebsd_carp_migration_candidate", "freebsd_nfs_migration_candidate", "freebsd_relayd_migration_candidate", "rnodes_nfs_client_migration_candidate", "system_nfs_client_migration_candidate"} {
  ops, err := api.RecordPlan("offline-storage", "", task); if err != nil { panic(err) }
  files := 0
  for _, op := range ops {
   switch op.Op {
   case plan.KindPlan, plan.KindWhenBegin, plan.KindWhenEnd:
   case plan.KindFile:
    must(strings.HasSuffix(op.Path, sm.Suffix), "candidate owns active path")
    must(op.Mode == "0644", "candidate file mode")
    files++
   default: panic(fmt.Sprintf("candidate %s has runtime op %s", task, op.Op))
   }
  }
  must(files > 0, "candidate recorded no files")
 }
 fmt.Println("PASS: render properties, drift/duplicate/mixed-subnet rejection, five inactive file-only plans; no credentials supplied")
}
'''
env = os.environ.copy()
env['GONF_CONF_ROOT'] = str(root)
with tempfile.TemporaryDirectory(prefix='storage-offline-') as directory:
    path = Path(directory) / 'main.go'
    path.write_text(source)
    subprocess.run(['go', 'run', str(path)], cwd=root / 'gonf', env=env, check=True)
