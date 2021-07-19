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

package dock

import (
	"io"
	"net/url"

	"shanhu.io/misc/errcode"
	"shanhu.io/misc/tarutil"
)

// BuildImage builds a docker image using the given tarball stream,
// and assigns the given tag.
func BuildImage(c *Client, tag string, tarball io.Reader) error {
	sink := newStreamSink()
	q := make(url.Values)
	q.Add("t", tag)
	q.Add("nocache", "true")
	if err := c.post("build", q, tarball, sink); err != nil {
		return err
	}
	return sink.waitDone()
}

// BuildImageStream builds a docker image using the given tarball stream,
// and assigns the given tag.
func BuildImageStream(c *Client, tag string, files *tarutil.Stream) error {
	r := newWriteToReader(files)
	defer r.Close()

	if err := BuildImage(c, tag, r); err != nil {
		return err
	}
	if err := r.Join(); err != nil {
		return errcode.Annotate(err, "send in files")
	}
	return nil
}

// NewTarStream creates a stream with a docker file.
func NewTarStream(dockerfile string) *tarutil.Stream {
	ts := tarutil.NewStream()
	AddDockerfileToStream(ts, dockerfile)
	return ts
}

// AddDockerfileToStream adds a DockerFile of content with mode 0600.
func AddDockerfileToStream(s *tarutil.Stream, content string) {
	s.AddString("Dockerfile", tarutil.ModeMeta(0600), content)
}
