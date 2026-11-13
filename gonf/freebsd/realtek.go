package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Realtek switches re0 (Realtek RTL8168H on every f-host) from the in-tree
// re(4) to Realtek's vendor driver, net/realtek-re-kmod, on the hosts whose
// RealtekNIC sets VendorDriver (conf task t74, IQ
// topics/infra/f3s/hosts/re0-watchdog.md).
//
// Why: the in-tree re(4) on 15.1 stalls the chip's transmitter every few
// days ("re0: watchdog timeout", link down ~4 s while re(4) resets it). On
// f1 that turns into a ~6 s CARP BACKUP -> MASTER flap, because tap0 keeps
// the bridge vm-public (and the vhid on it) up through the reset. FreeBSD
// bug 166724 reporters fixed the same symptom with the vendor driver. The
// switch changes the driver only: no offload flags besides the existing
// -txcsum -tso (Uplink), so the effect stays attributable.
//
// How: the package installs /boot/modules/if_re.ko; three loader.conf lines
// load it at boot (its probe outranks the built-in driver, the interface
// is still re0) and keep Wake-on-LAN working. All of it takes effect at
// the next reboot; nothing is reloaded here. Rollback: VendorDriver false,
// deploy, reboot; a host the vendor driver left without network is rolled
// back at the loader prompt on its JetKVM ("disable-module if_re", then
// "boot"; a plain "set if_re_load=NO" does not stop a module that
// loader.conf already queued).
//
// No TX watchdog: the installed build (1102.01.1501000_2, rtl_bsd_drv
// cea3599) compiles its watchdog only for FreeBSD < 7, so a transmit stall
// is never reset by the driver: re0 stays dead until a reboot, Oerrs do
// not grow and FreebsdNicWatchdogReset cannot fire. Such a stall shows as
// the host (and its guest) unreachable and, on f1, possibly a lasting CARP
// MASTER (when receive dies too, as in the 2026-11-13 stall).
// The _3 build (rtl_bsd_drv 5a108ea, in kmods_latest only on 2026-11-13)
// adds "TX watchdog: stalled" recovery that counts Oerrs again; the daily
// pkg upgrade installs it once kmods_quarterly carries it.
//
// Upgrade guard: the kmod is built per minor release, but the loader would
// load a 15.1 build into a 15.2 kernel. pkg picks the kmods repo
// (kmods_*_<minor>) from the userland's minor release, and freebsd-update
// boots the new kernel before it installs the new userland, so pkg cannot
// fetch the matching build before that first reboot. gonf-realtek-kmod-check
// (Check) compares the package's FreeBSD_version with the kernel;
// LoaderConf writes the lines only after it passes, and
// unattended-upgrade-freebsd refuses a kernel reboot while it fails. That
// covers the unattended reboot only: a manual reboot, an f3sctl power
// cycle, a UPS shutdown or a panic boots whatever loader.conf says. So
// before a 15.x minor upgrade set VendorDriver false and deploy (LoaderConf
// drops the lines), upgrade, pkg upgrade once the new userland is in, then
// set it true again.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// RealtekNIC.
type Realtek struct {
	RequiresRoot
}

// RealtekNIC is an f-host's re0 driver choice (host data on every f-host,
// see gonf/cluster). VendorDriver true installs realtek-re-kmod and loads
// it at boot; false keeps the in-tree re(4) and removes the loader lines
// (the rollback). The package stays installed either way: without the
// loader lines the built-in driver claims the NIC.
type RealtekNIC struct {
	VendorDriver bool
}

const (
	realtekPackage = "realtek-re-kmod"
	realtekCheck   = "/usr/local/libexec/gonf-realtek-kmod-check"
)

// realtekLoadLine makes the loader load the vendor module. The comment names
// the upgrade guard, since a minor upgrade is when the line bites.
const realtekLoadLine = `if_re_load="YES"` + "\t" +
	`# Realtek vendor re(4), realtek-re-kmod (conf t74); 15.x minor upgrade: run gonf-realtek-kmod-check first (IQ re0-watchdog.md)`

// realtekNameLine points if_re at the package's module instead of the
// stock /boot/kernel/if_re.ko (the port's pkg-message gives both lines).
const realtekNameLine = `if_re_name="/boot/modules/if_re.ko"`

// realtekS5WolLine keeps the PHY powered in soft-off so the magic packet
// from f3sctl ("f3sctl power fN on", pi0/pi1) still wakes the host. The
// vendor driver's re_shutdown powers the PHY down unless hw.re.s5wol is set
// (the port builds it with enable_s5wol = n); the in-tree re(4) armed
// WOL_MAGIC on shutdown by default. Read-only tunable: loader.conf only.
const realtekS5WolLine = `hw.re.s5wol="1"` + "\t" +
	`# vendor re(4) powers the PHY down at shutdown otherwise: no Wake-on-LAN (f3sctl power on)`

// Check installs gonf-realtek-kmod-check, the upgrade guard, on every f-host.
// unattended-upgrade-freebsd calls it before a kernel reboot; it passes
// where loader.conf does not load the vendor module.
func (Realtek) Check() {
	EnsureDir("/usr/local/libexec", RootOwned)
	InstallFile(realtekCheck, paths.FHostAsset("realtek/realtek-kmod-check"), RootExec)
}

// Package installs realtek-re-kmod (/boot/modules/if_re.ko) where
// RealtekNIC.VendorDriver is set.
//
// It comes from FreeBSD-ports-kmods (kmods_quarterly_<minor>), the repo pkg
// selects by the userland's minor release; that moves only after the reboot
// into a new minor's kernel, hence the upgrade guard (see Realtek).
func (Realtek) Package() {
	EachHost(func(n RealtekNIC) {
		if n.VendorDriver {
			Packages(realtekPackage)
		}
	})
}

// OptsLoaderConf orders the loader lines after the module and its guard.
func (Realtek) OptsLoaderConf() TaskOptions {
	return TaskOptions{Needs(Realtek.Check, Realtek.Package)}
}

// DescLoaderConf returns the description for the vendor driver lines.
func (Realtek) DescLoaderConf() string {
	return `loader.conf if_re_load/if_re_name/hw.re.s5wol where RealtekNIC.VendorDriver, removed elsewhere (next boot)`
}

// LoaderConf writes the vendor driver lines where RealtekNIC.VendorDriver
// is set and removes them elsewhere (the rollback; only lines spelled as
// written here are removed, a hand-edited variant stays).
//
// The lines are written only after "gonf-realtek-kmod-check -r" passed:
// the package is installed, if_re.ko is its intact file, and it was built
// for the minor release of both the installed and the running kernel. So a
// host never points the loader at a missing or mismatched module; the
// task fails instead. Read at boot; nothing is reloaded.
func (Realtek) LoaderConf() {
	EachHost(func(n RealtekNIC) {
		if !n.VendorDriver {
			File(loaderConf, WithoutLines(realtekLoadLine, realtekNameLine, realtekS5WolLine),
				RootOwned, WithName("loader-conf-realtek-absent"))
			return
		}
		ready := Command(realtekCheck, List("-r"), WithName("realtek-kmod-ready"))
		File(loaderConf,
			WithKeyedLine("if_re_load=", realtekLoadLine),
			WithKeyedLine("if_re_name=", realtekNameLine),
			WithKeyedLine("hw.re.s5wol=", realtekS5WolLine),
			RootOwned, WithName("loader-conf-realtek"), DependsOn(ready))
	})
}
