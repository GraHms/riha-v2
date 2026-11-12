package webhooks

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

type HTTPSender struct{ Client *http.Client }

var ErrBlockedAddress = errors.New("destination address is not allowed")

func blocked(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() ||
		(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1]&0xC0 == 64)
}

func NewHTTPSender(allowPrivate bool) *HTTPSender {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || blocked(ip) {
				return ErrBlockedAddress
			}
			return nil
		}
	}
	tr := &http.Transport{DialContext: dialer.DialContext, Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, MaxIdleConns: 50, IdleConnTimeout: 60 * time.Second}
	return &HTTPSender{Client: &http.Client{Timeout: 5 * time.Second, Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (s *HTTPSender) Send(ctx context.Context, url string, headers map[string]string, body []byte) (int, error) {
	code, _, err := s.SendReply(ctx, url, headers, body)
	return code, err
}

// SendReply sends a webhook and returns the first 4 KB of the reply.
func (s *HTTPSender) SendReply(ctx context.Context, url string, headers map[string]string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	reply, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, reply, nil
}
