package etchosts

//go:generate sh -c "go run ../cmd/hosts-migration -profile accepted -router-view upstream -format authoritative-hosts > ../../frontends/var/nsd/zones/master/accepted-lan.inc"
//go:generate sh -c "go run ../cmd/hosts-migration -profile accepted -router-view upstream -format hosts > ../../frontends/etc/hosts-fleet.inc"
