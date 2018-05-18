package doh

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/miekg/dns"
)

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

		query := new(dns.Msg)
		query.Question = []dns.Question{
			dns.Question{
				Name:   name,
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			},
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
