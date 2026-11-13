package rocky

import (
	. "github.com/snonux/gonf/api"
)

// PowerButton makes an ACPI power-button press that a Rocky bhyve guest
// receives while it boots power the guest off once it is up (task j74).
//
// vm-bhyve's "vm stop" (and "vm stopall" at the f-host's shutdown) sends
// bhyve SIGTERM twice, one second apart, and never again. Once the guest
// kernel has switched ACPI on, bhyve turns each SIGTERM into one ACPI
// power-button press; before that (UEFI, GRUB: the first ~8 s after "vm
// start" on f3) bhyve has no handler and the signal kills it outright, a
// hard power-off (exit status 143). systemd-logind (HandlePowerKey defaults
// to poweroff) acts on a press only once it watches /dev/input/event0,
// about 4 s after the kernel starts. A press in between is counted in
// /sys/firmware/acpi/interrupts/ff_pwr_btn, but its KEY_POWER input event
// has no reader and is lost. On 2026-11-12 a "vm stop rocky" sent 12 s
// after f3 started the guest landed there (logind watched 1 s later, per
// the guest's /var/log/messages) and left it running for 53 minutes; the
// reproduction on 2026-11-13 kept it running with ff_pwr_btn at 2. With the
// guest up, the same stop powers it off in 3 s.
//
// The recipe installs a oneshot unit ordered after systemd-logind. A
// nonzero ff_pwr_btn count at that point is a press logind may have missed,
// so the unit powers off. A press after the check reaches logind itself,
// and a press logind did handle only makes the poweroff request twice. The
// counter is per boot, so a guest-internal reboot starts at zero.
//
// RemainAfterExit plus IgnoreOnIsolate keep the unit active for the rest of
// the boot, so a later start of multi-user.target (also an isolate to
// rescue.target and back) does not run the check again against presses
// logind already saw. A guest booted straight into rescue mode runs it on
// its first multi-user start; a press made in rescue mode (logind stopped)
// then powers the guest off, which is what the press asked for.
//
// The unit is only enabled (WithEnableOnly), never started by an apply: on
// a running guest the counter may still hold presses from its own boot
// (the 2026-11-13 reproduction left it at 2), and starting the unit would
// power the guest off.
//
// Hosts opt in with WithData(BhyveGuest{}) in gonf/cluster; others are
// skipped (EachHostWith). Register with OnCluster(cluster.NameRockyAll).
type PowerButton struct{ RequiresRoot }

// BhyveGuest marks a Rocky host that runs as a vm-bhyve guest and is
// stopped with an ACPI power-button press (host data, see gonf/cluster).
type BhyveGuest struct{}

// powerButtonUnit is the oneshot that catches a press logind missed.
const powerButtonUnit = "power-button-catch-up"

// powerButtonUnitContent is the unit file. "$$" is a literal "$" for
// systemd; read strips the counter line's leading blanks.
const powerButtonUnitContent = `# Managed by gonf (rocky_power_button_unit, task j74).
[Unit]
Description=Power off for an ACPI power-button press made before logind watched the button
After=systemd-logind.service
IgnoreOnIsolate=yes
ConditionPathExists=/sys/firmware/acpi/interrupts/ff_pwr_btn

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c 'read -r n _ < /sys/firmware/acpi/interrupts/ff_pwr_btn; [ "$${n:-0}" -gt 0 ] || exit 0; echo "ff_pwr_btn=$$n before logind watched the power button: powering off"; exec systemctl --no-block poweroff'

[Install]
WantedBy=multi-user.target
`

// Unit installs and enables (without starting) the boot-time power-button
// catch-up unit on the hosts marked BhyveGuest.
func (PowerButton) Unit() {
	EachHostWith(func(BhyveGuest) {
		unit := File("/etc/systemd/system/"+powerButtonUnit+".service",
			WithContent(powerButtonUnitContent), RootOwned)
		reload := DaemonReload(OnChange(unit))
		Service(powerButtonUnit, WithEnableOnly, DependsOn(reload))
	})
}
