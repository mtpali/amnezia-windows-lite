package manager

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/ipc"
	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/amnezia-vpn/amneziawg-windows/v3/tunnel"
	"golang.org/x/sys/windows"
)

// Exercise the production listener, dialer, cached connection, UAPI parser and
// manager response over a real Windows named pipe without installing a VPN.
func TestLiteRuntimeTrafficOverNamedPipe(t *testing.T) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	owner := user.User.Sid
	// CI runs under its test account rather than SYSTEM. Production retains
	// its SYSTEM owner and administrator-only descriptor and owner checks.
	sd, err := windows.SecurityDescriptorFromString("O:" + owner.String() + "D:P(A;;GA;;;" + owner.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	originalSD := ipc.UAPISecurityDescriptor
	ipc.UAPISecurityDescriptor = sd
	defer func() { ipc.UAPISecurityDescriptor = originalSD }()
	name := fmt.Sprintf("traffic-%d", os.Getpid())
	listener, err := tunnel.ListenUAPI(name)
	if err != nil {
		t.Fatal("could not open the production tunnel pipe", err)
	}
	conf.PresetRootDirectory(t.TempDir())
	t.Cleanup(func() { conf.PresetRootDirectory("") })
	c := conf.Config{Name: name, Peers: []conf.Peer{{Endpoint: conf.Endpoint{Host: "192.0.2.1", Port: 51820}}}}
	c.Interface.PrivateKey[0] = 1
	c.Peers[0].PublicKey[0] = 2
	if err := c.Save(false); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	const rx, tx = uint64(1<<33 + 41), uint64(1<<32 + 7)
	const seconds, nanos = uint64(1700000000), uint64(123456789)
	response := func(step uint64) string {
		return fmt.Sprintf("private_key=%s\nrandom_trailers=0\ndisable_cookies=0\npublic_key=%s\nprotocol_version=1\nendpoint=192.0.2.1:51820\nlast_handshake_time_sec=%d\nlast_handshake_time_nsec=%d\ntx_bytes=%d\nrx_bytes=%d\nerrno=0\n\n", c.Interface.PrivateKey.HexString(), c.Peers[0].PublicKey.HexString(), seconds, nanos, tx+step*2048, rx+step*4096)
	}
	var requests atomic.Uint64
	done, serverErrors := make(chan struct{}), make(chan error, 1)
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					serverErrors <- err
				}
				return
			}
			err = func() error {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					command, err := reader.ReadString('\n')
					if err != nil {
						if errors.Is(err, io.EOF) {
							return nil
						}
						return err
					}
					blank, err := reader.ReadString('\n')
					if err != nil || command != "get=1\n" || blank != "\n" {
						return fmt.Errorf("invalid UAPI request %q %q: %v", command, blank, err)
					}
					step := requests.Add(1)
					data := response(step)
					if step == 3 {
						data = "errno=5\n\n"
					}
					if _, err := io.WriteString(conn, data); err != nil {
						return err
					}
				}
			}()
			if err != nil {
				serverErrors <- err
				return
			}
		}
	}()
	t.Cleanup(func() {
		disconnectTunnelServicePipe(name)
		listener.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("pipe server did not stop")
		}
		select {
		case err := <-serverErrors:
			t.Error(err)
		default:
		}
	})
	connectAsTestOwner := func() {
		t.Helper()
		var conn net.Conn
		var err error
		deadline := time.Now().Add(2 * time.Second)
		for {
			conn, err = dialTunnelServicePipe(name, owner)
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if err != nil {
			t.Fatal("manager could not reach the tunnel listener", err)
		}
		// Seed the normal cache with a connection checked by the production
		// dialer against the account that owns this test-only listener.
		connectedTunnelServicePipesLock.Lock()
		connectedTunnelServicePipes[name] = &connectedTunnel{Conn: conn}
		connectedTunnelServicePipesLock.Unlock()
	}
	connectAsTestOwner()
	service := &ManagerService{}
	checkSample := func(step uint64) {
		t.Helper()
		live, err := service.RuntimeConfig(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(live.Peers) != 1 {
			t.Fatalf("missing live peer: %+v", live)
		}
		p := live.Peers[0]
		if uint64(p.RxBytes) != rx+step*4096 || uint64(p.TxBytes) != tx+step*2048 || uint64(p.LastHandshakeTime) != seconds*uint64(time.Second)+nanos {
			t.Fatalf("live counters were lost or truncated: rx=%d tx=%d handshake=%d", p.RxBytes, p.TxBytes, p.LastHandshakeTime)
		}
	}
	checkSample(1)
	checkSample(2)
	if _, err := service.RuntimeConfig(name); err == nil {
		t.Fatal("failed runtime response was accepted as zero-byte statistics")
	}
	connectedTunnelServicePipesLock.RLock()
	_, cached := connectedTunnelServicePipes[name]
	connectedTunnelServicePipesLock.RUnlock()
	if cached {
		t.Fatal("failed runtime response left a stale pipe cached")
	}
	connectAsTestOwner()
	checkSample(4)
}
