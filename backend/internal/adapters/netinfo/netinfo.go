// Package netinfo names the network an IP address belongs to, such as Vodacom or
// Movitel, from a table built into RIHA, so no address leaves the server to be
// looked up. The table comes from iptoasn.com, public domain (PDDL v1.0); refresh
// it with deploy/tools/build-ipasn.py.
package netinfo

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"sort"
	"sync"
)

//go:embed ipasn.bin.gz
var table []byte

// Network is who carries an address.
type Network struct {
	ASN     uint32 `json:"asn"`
	Country string `json:"country"`
	Name    string `json:"name"`
	// Label is the short name people use, such as Vodacom. Kind says what sort of
	// network it is, when we know: mobile, fixed, relay, cloud or satellite.
	Label string `json:"label"`
	Kind  string `json:"kind,omitempty"`
}

type rng struct{ first, last, net uint32 }

var (
	once   sync.Once
	nets   []Network
	ranges []rng
)

// Known networks, by number, with the name people know them by.
var known = map[uint32][2]string{
	37223: {"Vodacom", "mobile"}, 37342: {"Movitel", "mobile"}, 30619: {"Tmcel", "mobile"}, 36945: {"Tmcel", "mobile"},
	25139: {"TVCabo", "fixed"}, 37697: {"Webmasters", "fixed"}, 42235: {"IDC", "fixed"}, 327700: {"MoRENet", "fixed"},
	328274: {"BIM (banco)", "fixed"}, 329769: {"SIMO", "fixed"},
	13335: {"Cloudflare (relay, por exemplo iCloud Private Relay)", "relay"}, 714: {"Apple", "relay"}, 6185: {"Apple", "relay"},
	15169: {"Google", "cloud"}, 396982: {"Google Cloud", "cloud"}, 16509: {"Amazon", "cloud"}, 14618: {"Amazon", "cloud"},
	8075: {"Microsoft", "cloud"}, 14061: {"DigitalOcean", "cloud"}, 24940: {"Hetzner", "cloud"}, 16276: {"OVH", "cloud"},
	14593: {"Starlink", "satellite"}, 32934: {"Meta", "cloud"},
}

// Load reads the table. It runs once, on the first lookup, or ahead of time.
func Load() {
	once.Do(func() {
		if err := load(); err != nil {
			log.Printf("netinfo: the network table could not be read: %v", err)
		}
	})
}

func load() error {
	zr, err := gzip.NewReader(bytes.NewReader(table))
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(raw, []byte("IPASN1")) {
		return errors.New("not a network table")
	}
	r := bytes.NewReader(raw[6:])
	var n uint32
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return err
	}
	out := make([]Network, n)
	for i := range out {
		var asn uint32
		cc := make([]byte, 2)
		if err := binary.Read(r, binary.BigEndian, &asn); err != nil {
			return err
		}
		if _, err := io.ReadFull(r, cc); err != nil {
			return err
		}
		l, err := r.ReadByte()
		if err != nil {
			return err
		}
		name := make([]byte, l)
		if _, err := io.ReadFull(r, name); err != nil {
			return err
		}
		nw := Network{ASN: asn, Country: string(cc), Name: string(name), Label: string(name)}
		if k, ok := known[asn]; ok {
			nw.Label, nw.Kind = k[0], k[1]
		}
		out[i] = nw
	}
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return err
	}
	rest := raw[len(raw)-r.Len():]
	if uint64(len(rest)) < uint64(n)*12 {
		return errors.New("the network table is cut short")
	}
	rs := make([]rng, n)
	for i := range rs {
		b := rest[i*12:]
		rs[i] = rng{binary.BigEndian.Uint32(b), binary.BigEndian.Uint32(b[4:]), binary.BigEndian.Uint32(b[8:])}
	}
	nets, ranges = out, rs
	return nil
}

// Lookup names the network of an IPv4 address. IPv6 and private addresses are not known.
func Lookup(ip string) (Network, bool) {
	Load()
	p := net.ParseIP(ip).To4()
	if p == nil || len(ranges) == 0 {
		return Network{}, false
	}
	v := binary.BigEndian.Uint32(p)
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i].first > v }) - 1
	if i < 0 || v > ranges[i].last {
		return Network{}, false
	}
	return nets[ranges[i].net], true
}
