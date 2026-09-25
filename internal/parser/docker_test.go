package parser

import (
	"strings"
	"testing"
)

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

// Compose allows a list of network names and long-syntax ports and
// volumes; any of them used to fail the whole data source.
func TestParseDockerComposeAcceptsLongSyntax(t *testing.T) {
	compose := []byte(`
services:
  web:
    image: nginx:1.27
    networks: [front, back]
    ports:
      - "8080:80"
      - 9000
      - target: 443
        published: 8443
        protocol: tcp
      - target: 53
        published: "5353"
        protocol: udp
        host_ip: 127.0.0.1
    volumes:
      - ./html:/usr/share/nginx/html:ro
      - type: bind
        source: ./conf
        target: /etc/nginx/conf.d
        read_only: true
      - type: tmpfs
        target: /tmp
  db:
    image: postgres:16
    networks:
      back:
        ipv4_address: 172.20.0.5
      front:
`)
	dc, err := ParseDockerCompose(compose)
	if err != nil {
		t.Fatal(err)
	}
	if len(dc.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(dc.Services))
	}
	db, web := dc.Services[0], dc.Services[1]

	if got := strings.Join(web.Networks, ","); got != "back,front" {
		t.Errorf("web networks = %q", got)
	}
	if got := strings.Join(web.Ports, " "); got != "8080:80 9000 8443:443 127.0.0.1:5353:53/udp" {
		t.Errorf("web ports = %q", got)
	}
	if got := strings.Join(web.Volumes, " "); got != "./html:/usr/share/nginx/html:ro ./conf:/etc/nginx/conf.d:ro /tmp" {
		t.Errorf("web volumes = %q", got)
	}
	if db.IP != "172.20.0.5" || strings.Join(db.Networks, ",") != "back,front" {
		t.Errorf("db = %+v", db)
	}
}
