package main

import (
	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"strings"
	"testing"
)

const testConfig = `[Interface]
PrivateKey = AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=
Address = 10.20.0.2/32
%s
[Peer]
PublicKey = AQECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=
Endpoint = 192.0.2.1:51820
AllowedIPs = 0.0.0.0/0, ::/0
`

func parsedConfig(t *testing.T, option string) *conf.Config {
	t.Helper()
	c, err := conf.FromWgQuick(strings.Replace(testConfig, "%s", option, 1), "sample")
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestLiteKillSwitchDefaultOff(t *testing.T) {
	c := parsedConfig(t, "")
	if c.Interface.BlockUntunneledTraffic || c.BlocksUntunneledTraffic() {
		t.Fatal("a standard full-tunnel import must not enable the kill switch")
	}
	for _, routes := range []string{"0.0.0.0/0", "::/0"} {
		if !strings.Contains(c.ToWgQuick(), routes) {
			t.Fatal("routing was changed", c.ToWgQuick())
		}
	}
	uapi, err := c.ToUAPI()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(uapi), "blockuntunneledtraffic") {
		t.Fatal("local policy must not be sent to the protocol engine")
	}
}
func TestLiteKillSwitchExplicitOptIn(t *testing.T) {
	for _, value := range []string{"on", "true", "1"} {
		c := parsedConfig(t, "BlockUntunneledTraffic = "+value)
		if !c.BlocksUntunneledTraffic() {
			t.Fatal("explicit opt-in was ignored", value)
		}
		roundTrip, err := conf.FromWgQuick(c.ToWgQuick(), c.Name)
		if err != nil || !roundTrip.BlocksUntunneledTraffic() {
			t.Fatal("opt-in was not preserved", err)
		}
		c.Interface.TableOff = true
		if c.BlocksUntunneledTraffic() {
			t.Fatal("Table=off must not install blocking rules")
		}
	}
	for _, value := range []string{"off", "false", "0"} {
		if parsedConfig(t, "BlockUntunneledTraffic = "+value).BlocksUntunneledTraffic() {
			t.Fatal(value)
		}
	}
	if _, err := conf.FromWgQuick(strings.Replace(testConfig, "%s", "BlockUntunneledTraffic = maybe", 1), "sample"); err == nil {
		t.Fatal("invalid policy accepted")
	}
}
func TestLiteKillSwitchEligibility(t *testing.T) {
	c := parsedConfig(t, "BlockUntunneledTraffic = on")
	c.Peers = append(c.Peers, c.Peers[0])
	if c.BlocksUntunneledTraffic() {
		t.Fatal("multiple peers must not engage automatic blocking")
	}
	c.Peers = c.Peers[:1]
	c.Peers[0].AllowedIPs[0].Cidr = 24
	c.Peers[0].AllowedIPs[1].Cidr = 64
	if c.BlocksUntunneledTraffic() {
		t.Fatal("split tunnel must not engage automatic blocking")
	}
}
