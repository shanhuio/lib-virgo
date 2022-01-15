// Copyright (C) 2022  Shanhu Tech Inc.
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
	"context"
	"net/url"

	"github.com/gorilla/websocket"
)

// DialOption provides addition option for dialing.
type DialOption struct {
	Dialer        *websocket.Dialer
	Token         string
	TunnelOptions *Options
}

// Dial connects to fabrics server, establishes a tunnel and returns an
// endpoint.
func Dial(
	ctx context.Context, addr *url.URL, opt *DialOption,
) (*Endpoint, error) {
	if opt == nil {
		opt = &DialOption{}
	}

	dialer := &websocketDialer{
		url:    addr,
		token:  opt.Token,
		dialer: opt.Dialer,
	}
	tunnOpt := opt.TunnelOptions
	if tunnOpt == nil {
		tunnOpt = &Options{}
	}
	conn, err := dialer.dial(ctx, tunnOpt)
	if err != nil {
		return nil, err
	}
	return newEndpoint(conn, dialer, tunnOpt), nil
}
