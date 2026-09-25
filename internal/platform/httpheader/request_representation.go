package httpheader

import (
	"net/http"
	"strings"
)

var representationMetadataNames = [...]string{
	"Content-Encoding",
	"Content-Length",
	"ETag",
	"Digest",
	"Content-MD5",
	"Content-Range",
	"Content-Digest",
	"Repr-Digest",
	"Signature",
	"Signature-Input",
}

func NormalizeUpstreamRequestRepresentation(request *http.Request, finalBodyLength int64) {
	if request == nil {
		return
	}
	if request.Header == nil {
		request.Header = make(http.Header)
	} else {
		request.Header = request.Header.Clone()
	}
	StripRepresentationMetadata(request.Header)
	deleteField(request.Header, "Accept-Encoding")
	request.Header.Set("Accept-Encoding", "identity")
	request.ContentLength = finalBodyLength
}

// StripRepresentationMetadata removes metadata tied to an original HTTP body for both requests and responses.
func StripRepresentationMetadata(headers http.Header) {
	if headers == nil {
		return
	}
	for _, name := range representationMetadataNames {
		deleteField(headers, name)
	}
}

func deleteField(headers http.Header, target string) {
	for name := range headers {
		if strings.EqualFold(name, target) {
			delete(headers, name)
		}
	}
}
