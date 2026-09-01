package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"console/internal/errs"
	"console/internal/model"
)

// listConsumers 对应 ConsumersController.list
func (c *Controller) listConsumers(r *ghttp.Request) {
	writePaginated(r, c.Consumer.List(parseCommonPageQuery(r)))
}

// addConsumer 对应 ConsumersController.add
func (c *Controller) addConsumer(r *ghttp.Request) {
	var consumer model.Consumer
	parseBody(r, &consumer)
	validateConsumer(&consumer, false)
	writeCreated(r, c.Consumer.AddOrUpdate(&consumer))
}

// queryConsumer 对应 ConsumersController.query
func (c *Controller) queryConsumer(r *ghttp.Request) {
	writeGet(r, c.Consumer.Query(pathParam(r, "name")))
}

// updateConsumer 对应 ConsumersController.put
func (c *Controller) updateConsumer(r *ghttp.Request) {
	name := pathParam(r, "name")
	var consumer model.Consumer
	parseBody(r, &consumer)
	if isBlank(consumer.Name) {
		consumer.Name = strPtr(name)
	} else if derefStr(consumer.Name) != name {
		panic(errs.Validation("Consumer name in the URL doesn't match the one in the body."))
	}
	validateConsumer(&consumer, true)
	writeUpdated(r, c.Consumer.AddOrUpdate(&consumer))
}

// deleteConsumer 对应 ConsumersController.delete
func (c *Controller) deleteConsumer(r *ghttp.Request) {
	c.Consumer.Delete(pathParam(r, "name"))
	writeNoContent(r)
}
