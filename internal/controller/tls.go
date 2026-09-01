package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// stripTlsSensitiveInfo 对应 TlsCertificatesController.stripSensitiveInfo
func stripTlsSensitiveInfo(t *model.TlsCertificate) {
	if t == nil {
		return
	}
	t.Cert = nil
	t.Key = nil
}

// listTlsCertificates 对应 TlsCertificatesController.list
func (c *Controller) listTlsCertificates(r *ghttp.Request) {
	result := c.Tls.List(parseCommonPageQuery(r))
	for i := range result.Data {
		stripTlsSensitiveInfo(&result.Data[i])
	}
	writePaginated(r, result)
}

// addTlsCertificate 对应 TlsCertificatesController.add
func (c *Controller) addTlsCertificate(r *ghttp.Request) {
	var certificate model.TlsCertificate
	parseBody(r, &certificate)
	result := c.Tls.Add(&certificate)
	stripTlsSensitiveInfo(result)
	writeCreated(r, result)
}

// queryTlsCertificate 对应 TlsCertificatesController.query
func (c *Controller) queryTlsCertificate(r *ghttp.Request) {
	result := c.Tls.Query(pathParam(r, "name"))
	stripTlsSensitiveInfo(result)
	writeGet(r, result)
}

// updateTlsCertificate 对应 TlsCertificatesController.put
func (c *Controller) updateTlsCertificate(r *ghttp.Request) {
	name := pathParam(r, "name")
	var certificate model.TlsCertificate
	parseBody(r, &certificate)
	if isEmpty(certificate.Name) {
		certificate.Name = strPtr(name)
	} else if derefStr(certificate.Name) != name {
		panic(errs.Validation("TlsCertificate name in the URL doesn't match the one in the body."))
	}
	result := c.Tls.Update(&certificate)
	stripTlsSensitiveInfo(result)
	writeUpdated(r, result)
}

// deleteTlsCertificate 对应 TlsCertificatesController.delete
func (c *Controller) deleteTlsCertificate(r *ghttp.Request) {
	c.Tls.Delete(pathParam(r, "name"))
	writeNoContent(r)
}
