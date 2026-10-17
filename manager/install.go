/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */

package manager

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/amnezia-vpn/amneziawg-windows/v3/services"
)

var cachedServiceManager *mgr.Mgr

func serviceManager() (*mgr.Mgr, error) {
	if cachedServiceManager != nil {
		return cachedServiceManager, nil
	}
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	cachedServiceManager = m
	return cachedServiceManager, nil
}

var ErrManagerAlreadyRunning = errors.New("Manager already installed and running")

func InstallManager() error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		return err
	}

	// TODO: Do we want to bail if executable isn't being run from the right location?

	serviceName := "WindowsLiteManager"
	service, err := m.OpenService(serviceName)
	if err == nil {
		status, err := service.Query()
		if err != nil {
			service.Close()
			return err
		}
		if status.State != svc.Stopped {
			service.Close()
			if status.State == svc.StartPending {
				// We were *just* started by something else, so return success here, assuming the other program
				// starting this does the right thing. This can happen when, e.g., the updater relaunches the
				// manager service and then invokes amneziawg.exe to raise the UI.
				return nil
			}
			return ErrManagerAlreadyRunning
		}
		err = service.Delete()
		service.Close()
		if err != nil {
			return err
		}
		for {
			service, err = m.OpenService(serviceName)
			if err != nil {
				break
			}
			service.Close()
			time.Sleep(time.Second / 3)
		}
	}

	config := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartManual,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  "amnezia Manager",
	}

	service, err = m.CreateService(serviceName, path, config, "/managerservice")
	if err != nil {
		return err
	}
	if err := service.Start(); err != nil {
		service.Close()
		return err
	}
	return service.Close()
}

func UninstallManager() error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	serviceName := "WindowsLiteManager"
	service, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	service.Control(svc.Stop)
	err = service.Delete()
	err2 := service.Close()
	if err != nil {
		return err
	}
	return err2
}

func InstallTunnel(configPath string) error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		return err
	}

	name, err := conf.NameFromPath(configPath)
	if err != nil {
		return err
	}

	serviceName, err := services.ServiceNameOfTunnel(name)
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err == nil {
		status, err := service.Query()
		if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			service.Close()
			return err
		}
		if status.State != svc.Stopped && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			service.Close()
			return errors.New("Tunnel already installed and running")
		}
		err = service.Delete()
		service.Close()
		if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			return err
		}
		for {
			service, err = m.OpenService(serviceName)
			if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				break
			}
			service.Close()
			time.Sleep(time.Second / 3)
		}
	}

	config := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartManual,
		ErrorControl: mgr.ErrorNormal,
		Dependencies: []string{"Nsi", "TcpIp"},
		DisplayName:  "amnezia Tunnel: " + name,
		SidType:      windows.SERVICE_SID_TYPE_UNRESTRICTED,
	}
	service, err = m.CreateService(serviceName, path, config, "/tunnelservice", configPath)
	if err != nil {
		return err
	}

	err = service.Start()
	go trackTunnelService(name, service) // Pass off reference to handle.
	return err
}

func UninstallTunnel(name string) error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	serviceName, err := services.ServiceNameOfTunnel(name)
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	service.Control(svc.Stop)
	err = service.Delete()
	err2 := service.Close()
	if err != nil && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
		return err
	}
	return err2
}

func changeTunnelServiceConfigFilePath(name, oldPath, newPath string) error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	serviceName, err := services.ServiceNameOfTunnel(name)
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err == windows.ERROR_SERVICE_DOES_NOT_EXIST {
		return nil
	} else if err != nil {
		return err
	}
	defer service.Close()
	config, err := service.Config()
	if err != nil {
		return err
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	args, err := windows.DecomposeCommandLine(config.BinaryPathName)
	if err != nil {
		return err
	}
	if len(args) != 3 || args[1] != "/tunnelservice" {
		return errors.New("Unexpected tunnel service command line")
	}
	if strings.EqualFold(args[2], newPath) {
		return nil
	}
	// The previous portable release used WindowsLite.exe in the same folder.
	sameExecutable := strings.EqualFold(args[0], exePath) ||
		(strings.EqualFold(filepath.Base(args[0]), "WindowsLite.exe") &&
			strings.EqualFold(filepath.Dir(args[0]), filepath.Dir(exePath)))
	if !sameExecutable || !strings.EqualFold(args[2], oldPath) {
		return errors.New("Tunnel service uses a different executable or configuration")
	}
	args[2] = newPath
	config.BinaryPathName = windows.ComposeCommandLine(args)
	return service.UpdateConfig(config)
}
