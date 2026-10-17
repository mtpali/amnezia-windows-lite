// SPDX-License-Identifier: MIT
package portable

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"golang.org/x/sys/windows"
)

func Directory() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func Configure() error {
	dir, err := Directory()
	if err != nil {
		return err
	}
	conf.PresetRootDirectory(filepath.Join(dir, "Data"))
	return nil
}

// The manager (SYSTEM) owns encrypted configs. Keep the same protected ACL as
// upstream, while placing the data next to the portable executable.
func PrepareDataDirectory() error {
	dir, err := conf.RootDirectory(false)
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("O:SYG:SYD:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	if err != nil {
		return err
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	if err = windows.CreateDirectory(path, sa); err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return err
	}
	h, err := windows.CreateFile(path, windows.READ_CONTROL|windows.WRITE_OWNER|windows.WRITE_DAC, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(h, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("Data must be a real directory, not a link")
	}
	return windows.SetKernelObjectSecurity(h, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, sd)
}
