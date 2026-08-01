package annette

import (
	"io"
	"net/http"
)

type (
	Response struct {
		res *http.Response
	}
)

func (r *Response) StatusCode() int {
	return r.res.StatusCode
}

func (r *Response) Body() string {
	return string(r.Binary())
}

func (r *Response) Binary() []byte {
	defer r.res.Body.Close()
	content, err := io.ReadAll(r.res.Body)
	if err != nil {
		return []byte{}
	}
	return content
}

func (r *Response) ContentLength() int64 {
	return r.res.ContentLength
}

func (r *Response) GetHeader() *Header {
	return &Header{hdr: r.res.Header}
}

func (r *Response) Proto() string {
	return r.res.Proto
}

func (r *Response) ProtoMajor() int {
	return r.res.ProtoMajor
}

func (r *Response) ProtoMinor() int {
	return r.res.ProtoMinor
}

func (r *Response) Close() bool {
	return r.res.Close
}

func (r *Response) Uncompressed() bool {
	return r.res.Uncompressed
}

func (r *Response) IsStatus100s() bool {
	return r.res.StatusCode < 200
}

func (r *Response) IsStatus200s() bool {
	return r.res.StatusCode >= 200 && r.res.StatusCode < 300
}

func (r *Response) IsStatus300s() bool {
	return r.res.StatusCode >= 300 && r.res.StatusCode < 400
}

func (r *Response) IsStatus400s() bool {
	return r.res.StatusCode >= 400 && r.res.StatusCode < 500
}

func (r *Response) IsStatus500s() bool {
	return r.res.StatusCode >= 500
}

func (r *Response) IsStatus100() bool {
	return r.res.StatusCode == http.StatusContinue
}

func (r *Response) IsStatus101() bool {
	return r.res.StatusCode == http.StatusSwitchingProtocols
}

func (r *Response) IsStatus102() bool {
	return r.res.StatusCode == http.StatusProcessing
}

func (r *Response) IsStatus103() bool {
	return r.res.StatusCode == http.StatusEarlyHints
}

func (r *Response) IsStatus200() bool {
	return r.res.StatusCode == http.StatusOK
}

func (r *Response) IsStatus201() bool {
	return r.res.StatusCode == http.StatusCreated
}

func (r *Response) IsStatus202() bool {
	return r.res.StatusCode == http.StatusAccepted
}

func (r *Response) IsStatus203() bool {
	return r.res.StatusCode == http.StatusNonAuthoritativeInfo
}

func (r *Response) IsStatus204() bool {
	return r.res.StatusCode == http.StatusNoContent
}

func (r *Response) IsStatus205() bool {
	return r.res.StatusCode == http.StatusResetContent
}

func (r *Response) IsStatus206() bool {
	return r.res.StatusCode == http.StatusPartialContent
}

func (r *Response) IsStatus207() bool {
	return r.res.StatusCode == http.StatusMultiStatus
}

func (r *Response) IsStatus208() bool {
	return r.res.StatusCode == http.StatusAlreadyReported
}

func (r *Response) IsStatus226() bool {
	return r.res.StatusCode == http.StatusIMUsed
}

func (r *Response) IsStatus300() bool {
	return r.res.StatusCode == http.StatusMultipleChoices
}

func (r *Response) IsStatus301() bool {
	return r.res.StatusCode == http.StatusMovedPermanently
}

func (r *Response) IsStatus302() bool {
	return r.res.StatusCode == http.StatusFound
}

func (r *Response) IsStatus303() bool {
	return r.res.StatusCode == http.StatusSeeOther
}

func (r *Response) IsStatus304() bool {
	return r.res.StatusCode == http.StatusNotModified
}

func (r *Response) IsStatus305() bool {
	return r.res.StatusCode == http.StatusUseProxy
}

func (r *Response) IsStatus307() bool {
	return r.res.StatusCode == http.StatusTemporaryRedirect
}

func (r *Response) IsStatus308() bool {
	return r.res.StatusCode == http.StatusPermanentRedirect
}

func (r *Response) IsStatus400() bool {
	return r.res.StatusCode == http.StatusBadRequest
}

func (r *Response) IsStatus401() bool {
	return r.res.StatusCode == http.StatusUnauthorized
}

func (r *Response) IsStatus402() bool {
	return r.res.StatusCode == http.StatusPaymentRequired
}

func (r *Response) IsStatus403() bool {
	return r.res.StatusCode == http.StatusForbidden
}

func (r *Response) IsStatus404() bool {
	return r.res.StatusCode == http.StatusNotFound
}

func (r *Response) IsStatus405() bool {
	return r.res.StatusCode == http.StatusMethodNotAllowed
}

func (r *Response) IsStatus406() bool {
	return r.res.StatusCode == http.StatusNotAcceptable
}

func (r *Response) IsStatus407() bool {
	return r.res.StatusCode == http.StatusProxyAuthRequired
}

func (r *Response) IsStatus408() bool {
	return r.res.StatusCode == http.StatusRequestTimeout
}

func (r *Response) IsStatus409() bool {
	return r.res.StatusCode == http.StatusConflict
}

func (r *Response) IsStatus410() bool {
	return r.res.StatusCode == http.StatusGone
}

func (r *Response) IsStatus411() bool {
	return r.res.StatusCode == http.StatusLengthRequired
}

func (r *Response) IsStatus412() bool {
	return r.res.StatusCode == http.StatusPreconditionFailed
}

func (r *Response) IsStatus413() bool {
	return r.res.StatusCode == http.StatusRequestEntityTooLarge
}

func (r *Response) IsStatus414() bool {
	return r.res.StatusCode == http.StatusRequestURITooLong
}

func (r *Response) IsStatus415() bool {
	return r.res.StatusCode == http.StatusUnsupportedMediaType
}

func (r *Response) IsStatus416() bool {
	return r.res.StatusCode == http.StatusRequestedRangeNotSatisfiable
}

func (r *Response) IsStatus417() bool {
	return r.res.StatusCode == http.StatusExpectationFailed
}

func (r *Response) IsStatus418() bool {
	return r.res.StatusCode == http.StatusTeapot
}

func (r *Response) IsStatus421() bool {
	return r.res.StatusCode == http.StatusMisdirectedRequest
}

func (r *Response) IsStatus422() bool {
	return r.res.StatusCode == http.StatusUnprocessableEntity
}

func (r *Response) IsStatus423() bool {
	return r.res.StatusCode == http.StatusLocked
}

func (r *Response) IsStatus424() bool {
	return r.res.StatusCode == http.StatusFailedDependency
}

func (r *Response) IsStatus425() bool {
	return r.res.StatusCode == http.StatusTooEarly
}

func (r *Response) IsStatus426() bool {
	return r.res.StatusCode == http.StatusUpgradeRequired
}

func (r *Response) IsStatus428() bool {
	return r.res.StatusCode == http.StatusPreconditionRequired
}

func (r *Response) IsStatus429() bool {
	return r.res.StatusCode == http.StatusTooManyRequests
}

func (r *Response) IsStatus431() bool {
	return r.res.StatusCode == http.StatusRequestHeaderFieldsTooLarge
}

func (r *Response) IsStatus451() bool {
	return r.res.StatusCode == http.StatusUnavailableForLegalReasons
}

func (r *Response) IsStatus500() bool {
	return r.res.StatusCode == http.StatusInternalServerError
}

func (r *Response) IsStatus501() bool {
	return r.res.StatusCode == http.StatusNotImplemented
}

func (r *Response) IsStatus502() bool {
	return r.res.StatusCode == http.StatusBadGateway
}

func (r *Response) IsStatus503() bool {
	return r.res.StatusCode == http.StatusServiceUnavailable
}

func (r *Response) IsStatus504() bool {
	return r.res.StatusCode == http.StatusGatewayTimeout
}

func (r *Response) IsStatus505() bool {
	return r.res.StatusCode == http.StatusHTTPVersionNotSupported
}

func (r *Response) IsStatus506() bool {
	return r.res.StatusCode == http.StatusVariantAlsoNegotiates
}

func (r *Response) IsStatus507() bool {
	return r.res.StatusCode == http.StatusInsufficientStorage
}

func (r *Response) IsStatus508() bool {
	return r.res.StatusCode == http.StatusLoopDetected
}

func (r *Response) IsStatus510() bool {
	return r.res.StatusCode == http.StatusNotExtended
}

func (r *Response) IsStatus511() bool {
	return r.res.StatusCode == http.StatusNetworkAuthenticationRequired
}
