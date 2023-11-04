{
  "StateDir": "/var/run/gorum",
  "Address": "<%= $hostname.'.'.$domain %>:4321",
  "Nodes": {
    "Blowfish": {
      "Hostname": "blowfish.buetow.org",
      "Port": 4321,
      "Priority": 100
    },
    "Fishfinger": {
      "Hostname": "fishfinger.buetow.org",
      "Port": 4321,
      "Priority": 50
    }
  }
}
