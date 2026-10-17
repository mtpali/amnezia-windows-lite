/* SPDX-License-Identifier: MIT
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

package tunnel

import (
	"net"

	"github.com/amnezia-vpn/amneziawg-go/v3/ipc"
	"github.com/amnezia-vpn/amneziawg-go/v3/ipc/namedpipe"
	"github.com/amnezia-vpn/amneziawg-windows/v3/services"
)

// ListenUAPI uses the same isolated pipe path as the manager. The engine's
// default listener hardcodes the upstream AmneziaWG namespace instead.
func ListenUAPI(name string) (net.Listener, error) {
	path, err := services.PipePathOfTunnel(name)
	if err != nil {
		return nil, err
	}
	return (&namedpipe.ListenConfig{
		SecurityDescriptor: ipc.UAPISecurityDescriptor,
	}).Listen(path)
}
