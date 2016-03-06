package web

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/teleport"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

const (
	// HTTPS is https prefix
	HTTPS = "https"
	// WSS is secure web sockets prefix
	WSS = "wss"
)

// SSHAgentLogin issues call to web proxy and receives temp certificate
// if credentials are valid
//
// proxyAddr must be specified as host:port
func SSHAgentLogin(proxyAddr, user, password, hotpToken string, pubKey []byte, ttl time.Duration, insecure bool) (*SSHLoginResponse, error) {
	// validate proxyAddr:
	host, port, err := net.SplitHostPort(proxyAddr)
	if err != nil || host == "" || port == "" {
		if err != nil {
			log.Error(err)
		}
		return nil, trace.Wrap(
			teleport.BadParameter("proxyAddress",
				fmt.Sprintf("'%v' is not a valid proxy address", proxyAddr)))
	}
	proxyAddr = "https://" + net.JoinHostPort(host, port)

	var opts []roundtrip.ClientParam
	if insecure {
		log.Warningf("you are using insecure HTTPS connection")
		opts = append(opts, roundtrip.HTTPClient(newInsecureClient()))
	} else {
		tlsconf := &tls.Config{}
		tlsconf.RootCAs = x509.NewCertPool()
		bytes, err := ioutil.ReadFile("/var/lib/teleport/webproxy_https.cert")
		if err != nil {
			panic(err)
		}
		block, _ := pem.Decode(bytes)
		if block == nil {
			panic("block is null")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			panic(err)
		}
		tlsconf.RootCAs.AddCert(cert)
		client := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsconf},
		}
		opts = append(opts, roundtrip.HTTPClient(client))
	}

	clt, err := newWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(clt.Endpoint("webapi", "ssh", "certs"), createSSHCertReq{
		User:      user,
		Password:  password,
		HOTPToken: hotpToken,
		PubKey:    pubKey,
		TTL:       ttl,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}
