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

// serverName returns the SNI server name inside the TLS ClientHello,
// without consuming any bytes from br.
// On any error, the empty string is returned.
func (c *bufConn) serverName() (string, error) {
	const headerLen = 5
	hdr, err := c.br.Peek(headerLen)
	if err != nil {
		return "", err
	}
	const handshake = 0x16
	if hdr[0] != handshake {
		return "", fmt.Errorf("not TLS: 0x%02x != 0x16", hdr[0]) // Not TLS.
	}
	recLen := int(hdr[3])<<8 | int(hdr[4]) // ignoring version in hdr[1:3]

	helloBytes, err := c.br.Peek(headerLen + recLen)
	if err != nil {
		return "", err
	}

	var name string
	tls.Server(
		&headerConn{r: bytes.NewReader(helloBytes)},
		nameSinkTLSConfig(&name),
	).Handshake()
	return name, nil
}

func nameSinkTLSConfig(name *string) *tls.Config {
	getConfig := func(h *tls.ClientHelloInfo) (*tls.Config, error) {
		*name = h.ServerName
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
