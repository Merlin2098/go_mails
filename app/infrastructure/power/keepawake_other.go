//go:build !windows

package power

// KeepAwake is a no-op outside Windows; the app only targets Windows, but
// keeping the package buildable everywhere lets tests run on any OS.
func KeepAwake() (release func()) {
	return func() {}
}
