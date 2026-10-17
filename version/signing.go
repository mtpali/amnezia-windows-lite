// SPDX-License-Identifier: MIT
package version

// Portable community builds use normal UAC elevation.
func IsRunningEVSigned() bool { return false }
