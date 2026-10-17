// SPDX-License-Identifier: MIT
package manager

// Reserved IPC values are retained for protocol stability. Portable builds do
// not contact an update service or download/install an MSI.
type UpdateState uint32

const (
	UpdateStateUnknown UpdateState = iota
	UpdateStateFoundUpdate
	UpdateStateUpdatesDisabledUnofficialBuild
)

type DownloadProgress struct {
	Activity        string
	BytesDownloaded uint64
	BytesTotal      uint64
	Error           error
	Complete        bool
}
