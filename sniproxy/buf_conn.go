// Copyright (C) 2021  Shanhu Tech Inc.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package sniproxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
)

type helloInfo struct {
	serverName string
	protoCount int
	firstProto string
}

type bufConn struct {
	net.Conn
	br *bufio.Reader
}

func newBufConn(conn net.Conn) *bufConn {
	return &bufConn{
		Conn: conn,
		br:   bufio.NewReader(conn),
	}
}

func (c *bufConn) Read(buf []byte) (int, error) {
	return c.br.Read(buf)
}

// helloInfo returns the SNI information extracted from the TLS ClientHello,
// without consuming any bytes from br.
// On any error, the empty string is returned.
func (c *bufConn) helloInfo() (*helloInfo, error) {
	const headerLen = 5
	hdr, err := c.br.Peek(headerLen)
	if err != nil {
		return nil, err
	}
	const handshake = 0x16
	if hdr[0] != handshake {
		return nil, fmt.Errorf("not TLS: 0x%02x != 0x16", hdr[0]) // Not TLS.
	}
	recLen := int(hdr[3])<<8 | int(hdr[4]) // ignoring version in hdr[1:3]

	helloBytes, err := c.br.Peek(headerLen + recLen)
	if err != nil {
		return nil, err
	}

	info := new(helloInfo)
	tls.Server(
		&headerConn{r: bytes.NewReader(helloBytes)},
		nameSinkTLSConfig(info),
	).Handshake()
	return info, nil
}

func nameSinkTLSConfig(info *helloInfo) *tls.Config {
	getConfig := func(h *tls.ClientHelloInfo) (*tls.Config, error) {
		info.serverName = h.ServerName
		info.protoCount = len(h.SupportedProtos)
		if info.protoCount > 0 {
			info.firstProto = h.SupportedProtos[0]
		}
		return nil, nil
	}
	return &tls.Config{GetConfigForClient: getConfig}
}

type headerConn struct {
	net.Conn // just to implement other methods in the inteface.
	r        io.Reader
}

func (c *headerConn) Read(p []byte) (int, error) { return c.r.Read(p) }
func (*headerConn) Write(p []byte) (int, error)  { return 0, io.EOF }
