package runtime

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// muxiRuntimePattern matches the muxi_runtime field value with an optional
// :<variant> suffix.
//
//	<version>              -> e.g. "latest", "0.20260422.0", "1.2.3"
//	<version>:<variant>    -> e.g. "latest:pytorch", "0.20260422.0:pytorch"
//
// The version half is intentionally permissive (the literal "latest" or a
// dotted integer sequence) to preserve back-compat with formations that pin
// semver-style runtimes (e.g. "1.2.3") as well as calver-style runtimes
// (e.g. "0.20260422.0"). Narrowing the version regex would break existing
// formations in the wild; validation of the version half is left to the
// downstream Resolver that already handles it.
//
// The variant half is strictly kebab-lowercase so garbage tokens (spaces,
// uppercase, special chars) fail fast at parse time rather than producing
// a confusing "SIF not found on disk" error later in the deploy path.
var muxiRuntimePattern = regexp.MustCompile(
	`^(latest|[0-9]+(?:\.[0-9]+)*)(?::([a-z][a-z0-9-]*))?$`,
)

// DefaultVariant is the SIF variant used when muxi_runtime omits a variant.
// "lean" is the zero-config default: ONNX-only, smallest SIF, covers the
// common case (Nomic v1.5, MiniLM family, bge-*, Arctic, GTE). Keeping it
// suffix-free on disk also preserves compatibility with historical SIF
// artifact naming (no -variant suffix before this field existed).
const DefaultVariant = "lean"

// ValidVariants is the allowlist of recognized SIF variants. Variants outside
// this set are rejected at parse time — accepting them would only defer the
// failure to a less-actionable "SIF not found on disk" error in artifact
// selection (S2). The set grows when new variants are actually built and
// shipped from the runtime repo.
//
// Reserved-but-not-yet-built: "gpu", "pytorch-gpu". They are intentionally
// NOT in this map so a formation requesting them today gets a clear error.
var ValidVariants = map[string]struct{}{
	"lean":    {},
	"pytorch": {},
}

// ParseMuxiRuntime splits a muxi_runtime field value into (version, variant).
//
// Normalization (matches the S1 table in the embedding SIF deployment plan):
//
//	""                         -> ("latest", "lean")
//	"latest"                   -> ("latest", "lean")
//	"0.20260422.0"             -> ("0.20260422.0", "lean")
//	"latest:pytorch"           -> ("latest", "pytorch")
//	"0.20260422.0:pytorch"     -> ("0.20260422.0", "pytorch")
//	"latest:unknown"           -> error (variant not in allowlist)
//	"0.20260422.0:"            -> error (regex mismatch on empty variant)
//	"garbage"                  -> error (regex mismatch)
//
// The returned version is passed through to Resolver.Resolve unchanged. The
// returned variant is consumed downstream when selecting the SIF artifact
// on disk (S2) and when bind-mounting the host HF cache into the SIF (S4).
func ParseMuxiRuntime(s string) (version, variant string, err error) {
	if s == "" {
		return "latest", DefaultVariant, nil
	}

	m := muxiRuntimePattern.FindStringSubmatch(s)
	if m == nil {
		return "", "", fmt.Errorf(
			"invalid muxi_runtime %q: expected \"<version>[:<variant>]\" "+
				"where version is \"latest\" or a dotted integer and variant is lowercase kebab",
			s,
		)
	}

	version = m[1]
	variant = m[2]
	if variant == "" {
		variant = DefaultVariant
	}

	if _, ok := ValidVariants[variant]; !ok {
		return "", "", fmt.Errorf(
			"invalid muxi_runtime variant %q in %q: allowed variants are %s",
			variant, s, allowedVariantsList(),
		)
	}

	return version, variant, nil
}

// allowedVariantsList returns a stable, comma-separated, quoted list of valid
// variants for inclusion in error messages. Sorted alphabetically so test
// expectations and user-facing errors are deterministic across runs.
func allowedVariantsList() string {
	names := make([]string, 0, len(ValidVariants))
	for v := range ValidVariants {
		names = append(names, v)
	}
	sort.Strings(names)
	for i, n := range names {
		names[i] = `"` + n + `"`
	}
	return strings.Join(names, ", ")
}
