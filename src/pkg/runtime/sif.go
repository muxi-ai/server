package runtime

import "fmt"

// sifFilename constructs the on-disk SIF filename for a given (version, variant)
// pair. It is the single source of truth for the SIF naming convention; both
// the Resolver (for path lookup) and the Downloader (for URL + destination
// construction) must route through it.
//
// Naming convention (variant-before-platform):
//
//	(version, "lean")         -> muxi-runtime-<version>-linux-<arch>.sif
//	(version, "pytorch")      -> muxi-runtime-<version>-pytorch-linux-<arch>.sif
//	(version, "gpu")          -> muxi-runtime-<version>-gpu-linux-<arch>.sif         (future)
//	(version, "pytorch-gpu")  -> muxi-runtime-<version>-pytorch-gpu-linux-<arch>.sif (future)
//
// The default (lean) is intentionally suffix-free so existing release
// artifacts, deploy scripts, and operator muscle memory keep working without
// migration. Non-default variants insert their name before the platform
// segment — alphabetically groups related SIFs on disk and reads naturally
// (version, then build variant, then platform).
//
// An empty variant is treated as DefaultVariant for defensive correctness;
// in practice callers route through ParseMuxiRuntime, which normalizes
// empty-or-missing to DefaultVariant before reaching here.
func sifFilename(version, variant string) string {
	if variant == "" {
		variant = DefaultVariant
	}
	platform := getPlatform()
	if variant == DefaultVariant {
		// Suffix-free: preserves the historical (pre-variant) naming so
		// existing SIFs on disk and existing release artifacts keep working.
		return fmt.Sprintf("muxi-runtime-%s-%s.sif", version, platform)
	}
	return fmt.Sprintf("muxi-runtime-%s-%s-%s.sif", version, variant, platform)
}
