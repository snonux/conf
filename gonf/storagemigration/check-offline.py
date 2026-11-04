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
 got, err := sm.Server(string(server)); must(err == nil, "server render")
 must(strings.ReplaceAll(got, sm.LabVIP, sm.OldVIP) == string(server), "server changed unrelated policy")
 for _, bad := range []string{"", got, string(server)+string(server)} {
  _, err = sm.Server(bad); must(err != nil, "server accepted drift")
 }
 got, err = sm.Relayd(string(relay)); must(err == nil, "relayd render")
 must(strings.ReplaceAll(got, "10.20.0.", "192.168.1.") == string(relay), "relayd changed unrelated config")
 for _, bad := range []string{"", got, strings.ReplaceAll(string(relay), "192.168.1.121", "192.168.1.120"), strings.ReplaceAll(string(relay), "192.168.1.121", "10.20.0.121"), string(relay)+string(relay)} {
  _, err = sm.Relayd(bad); must(err != nil, "relayd accepted drift, duplicate or mixed subnet")
 }
 for _, identity := range []string{"r0", "r1", "r2", "earth"} {
  original := "cert = /etc/stunnel/"+identity+"-stunnel.pem\nverify = 2\naccept = 127.0.0.1:2323\nconnect = "+sm.OldVIP+":2323\n"
  got, err = sm.Client(original); must(err == nil, "client render")
  must(strings.ReplaceAll(got, sm.LabVIP, sm.OldVIP) == original, "client identity drift")
  _, err = sm.Client(got); must(err != nil, "client accepted already promoted source")
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
