package parser

import "testing"

// With a static address on several networks, the service IP is the one on
// the first network by name, not whichever map iteration reaches last.
func TestParseDockerComposeIPIsDeterministic(t *testing.T) {
	compose := []byte(`
services:
  app:
    image: nginx
    networks:
      zulu: {ipv4_address: 10.0.0.26}
      alpha: {ipv4_address: 10.0.0.1}
      mike: {ipv4_address: 10.0.0.13}
      bravo: {ipv4_address: 10.0.0.2}
      kilo: {ipv4_address: 10.0.0.11}
      echo: {ipv4_address: 10.0.0.5}
      india: {ipv4_address: 10.0.0.9}
      oscar: {ipv4_address: 10.0.0.15}
      tango: {ipv4_address: 10.0.0.20}
`)
	for i := 0; i < 20; i++ {
		dc, err := ParseDockerCompose(compose)
		if err != nil {
			t.Fatal(err)
		}
		if got := dc.Services[0].IP; got != "10.0.0.1" {
			t.Fatalf("IP = %q, want the alpha network's 10.0.0.1", got)
		}
	}
}
