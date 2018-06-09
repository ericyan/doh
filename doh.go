package doh

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/miekg/dns"
)

type Handler struct {
	Upstream string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resolver := r.URL.Query().Get("resolver")
	if resolver == "" {
		resolver = h.Upstream
	}

	var f func(http.ResponseWriter, *http.Request)
	if r.Method == http.MethodGet && r.URL.Query().Get("dns") == "" {
		f = HandleJSON(resolver)
	} else {
		f = HandleWireFormat(resolver)
	}

	f(w, r)
}

func HandleWireFormat(upstream string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			buf []byte
			err error
		)

		switch r.Method {
		case http.MethodGet:
			buf, err = base64.RawURLEncoding.DecodeString(r.URL.Query().Get("dns"))
			if len(buf) == 0 || err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		case http.MethodPost:
			if r.Header.Get("Content-Type") != "application/dns-message" {
				http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
				return
			}

			buf, err = ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer r.Body.Close()
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		query := new(dns.Msg)
		if err := query.Unpack(buf); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		answer, err := dns.Exchange(query, upstream)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		packed, err := answer.Pack()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/dns-message")
		w.Write(packed)
	}
}

func HandleJSON(upstream string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if !strings.HasSuffix(name, ".") {
			buf := bytes.NewBufferString(name)
			buf.WriteString(".")
			name = buf.String()
		}

		qtype := ParseQTYPE(r.URL.Query().Get("type"))
		if qtype == dns.TypeNone {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		query := new(dns.Msg)
		query.Question = []dns.Question{
			dns.Question{
				Name:   name,
				Qtype:  qtype,
				Qclass: dns.ClassINET,
			},
		}

		if ecs := r.URL.Query().Get("edns_client_subnet"); ecs != "" {
			_, subnet, err := net.ParseCIDR(ecs)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			mask, bits := subnet.Mask.Size()
			var af uint16
			if bits == 32 {
				af = 1
			} else {
				af = 2
			}

			query.Extra = append(query.Extra, &dns.OPT{
				Hdr: dns.RR_Header{
					Name:   ".",
					Class:  dns.DefaultMsgSize,
					Rrtype: dns.TypeOPT,
				},
				Option: []dns.EDNS0{
					&dns.EDNS0_SUBNET{
						Code:          dns.EDNS0SUBNET,
						Family:        af,
						SourceNetmask: uint8(mask),
						SourceScope:   0,
						Address:       subnet.IP,
					},
				},
			})
		}

		answer, err := dns.Exchange(query, upstream)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		json, err := json.Marshal(NewMsg(answer))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/dns-json")
		w.Write(json)
	}
}
