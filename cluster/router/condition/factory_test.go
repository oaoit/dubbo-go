/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package condition

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
)

import (
	"github.com/dubbogo/gost/net"
	perrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

import (
	"github.com/apache/dubbo-go/common"
	"github.com/apache/dubbo-go/common/logger"
	"github.com/apache/dubbo-go/protocol"
	"github.com/apache/dubbo-go/protocol/invocation"
)

const anyUrl = "condition://0.0.0.0/com.foo.BarService"

type MockInvoker struct {
	url          common.URL
	available    bool
	destroyed    bool
	successCount int
}

func NewMockInvoker(url common.URL, successCount int) *MockInvoker {
	return &MockInvoker{
		url:          url,
		available:    true,
		destroyed:    false,
		successCount: successCount,
	}
}

func (bi *MockInvoker) GetUrl() common.URL {
	return bi.url
}

func getRouteUrl(rule string) *common.URL {
	url, _ := common.NewURL(anyUrl)
	url.AddParam("rule", rule)
	url.AddParam("force", "true")
	return &url
}

func getRouteUrlWithForce(rule, force string) *common.URL {
	url, _ := common.NewURL(anyUrl)
	url.AddParam("rule", rule)
	url.AddParam("force", force)
	return &url
}

func getRouteUrlWithNoForce(rule string) *common.URL {
	url, _ := common.NewURL(anyUrl)
	url.AddParam("rule", rule)
	return &url
}

func (bi *MockInvoker) IsAvailable() bool {
	return bi.available
}

func (bi *MockInvoker) IsDestroyed() bool {
	return bi.destroyed
}

type rest struct {
	tried   int
	success bool
}

var count int

func (bi *MockInvoker) Invoke(_ context.Context, _ protocol.Invocation) protocol.Result {
	count++

	var (
		success bool
		err     error
	)
	if count >= bi.successCount {
		success = true
	} else {
		err = perrors.New("error")
	}

	result := &protocol.RPCResult{Err: err, Rest: rest{tried: count, success: success}}
	return result
}

func (bi *MockInvoker) Destroy() {
	logger.Infof("Destroy invoker: %v", bi.GetUrl().String())
	bi.destroyed = true
	bi.available = false
}

func TestRouteMatchWhen(t *testing.T) {
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("=> host = 1.2.3.4"))
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	cUrl, _ := common.NewURL("consumer://1.1.1.1/com.foo.BarService")
	matchWhen := router.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, true, matchWhen)
	rule1 := base64.URLEncoding.EncodeToString([]byte("host = 2.2.2.2,1.1.1.1,3.3.3.3 => host = 1.2.3.4"))
	router1, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule1))
	matchWhen1 := router1.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, true, matchWhen1)
	rule2 := base64.URLEncoding.EncodeToString([]byte("host = 2.2.2.2,1.1.1.1,3.3.3.3 & host !=1.1.1.1 => host = 1.2.3.4"))
	router2, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule2))
	matchWhen2 := router2.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, false, matchWhen2)
	rule3 := base64.URLEncoding.EncodeToString([]byte("host !=4.4.4.4 & host = 2.2.2.2,1.1.1.1,3.3.3.3 => host = 1.2.3.4"))
	router3, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule3))
	matchWhen3 := router3.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, true, matchWhen3)
	rule4 := base64.URLEncoding.EncodeToString([]byte("host !=4.4.4.* & host = 2.2.2.2,1.1.1.1,3.3.3.3 => host = 1.2.3.4"))
	router4, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule4))
	matchWhen4 := router4.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, true, matchWhen4)
	rule5 := base64.URLEncoding.EncodeToString([]byte("host = 2.2.2.2,1.1.1.*,3.3.3.3 & host != 1.1.1.1 => host = 1.2.3.4"))
	router5, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule5))
	matchWhen5 := router5.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, false, matchWhen5)
	rule6 := base64.URLEncoding.EncodeToString([]byte("host = 2.2.2.2,1.1.1.*,3.3.3.3 & host != 1.1.1.2 => host = 1.2.3.4"))
	router6, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule6))
	matchWhen6 := router6.(*ConditionRouter).MatchWhen(&cUrl, inv)
	assert.Equal(t, true, matchWhen6)
}

func TestRouteMatchFilter(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	t.Logf("The local ip is %s", localIP)
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService?default.serialization=fastjson")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invokers := []protocol.Invoker{NewMockInvoker(url1, 1), NewMockInvoker(url2, 2), NewMockInvoker(url3, 3)}
	rule1 := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = 10.20.3.3"))
	rule2 := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = 10.20.3.* & host != 10.20.3.3"))
	rule3 := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = 10.20.3.3  & host != 10.20.3.3"))
	rule4 := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = 10.20.3.2,10.20.3.3,10.20.3.4"))
	rule5 := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host != 10.20.3.3"))
	rule6 := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " serialization = fastjson"))
	router1, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule1))
	router2, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule2))
	router3, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule3))
	router4, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule4))
	router5, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule5))
	router6, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule6))
	cUrl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	fileredInvokers1 := router1.Route(invokers, &cUrl, &invocation.RPCInvocation{})
	fileredInvokers2 := router2.Route(invokers, &cUrl, &invocation.RPCInvocation{})
	fileredInvokers3 := router3.Route(invokers, &cUrl, &invocation.RPCInvocation{})
	fileredInvokers4 := router4.Route(invokers, &cUrl, &invocation.RPCInvocation{})
	fileredInvokers5 := router5.Route(invokers, &cUrl, &invocation.RPCInvocation{})
	fileredInvokers6 := router6.Route(invokers, &cUrl, &invocation.RPCInvocation{})
	assert.Equal(t, 1, len(fileredInvokers1))
	assert.Equal(t, 0, len(fileredInvokers2))
	assert.Equal(t, 0, len(fileredInvokers3))
	assert.Equal(t, 1, len(fileredInvokers4))
	assert.Equal(t, 2, len(fileredInvokers5))
	assert.Equal(t, 1, len(fileredInvokers6))

}

func TestRouteMethodRoute(t *testing.T) {
	inv := invocation.NewRPCInvocationWithOptions(invocation.WithMethodName("getFoo"), invocation.WithParameterTypes([]reflect.Type{}), invocation.WithArguments([]interface{}{}))
	rule := base64.URLEncoding.EncodeToString([]byte("host !=4.4.4.* & host = 2.2.2.2,1.1.1.1,3.3.3.3 => host = 1.2.3.4"))
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	url, _ := common.NewURL("consumer://1.1.1.1/com.foo.BarService?methods=setFoo,getFoo,findFoo")
	matchWhen := router.(*ConditionRouter).MatchWhen(&url, inv)
	assert.Equal(t, true, matchWhen)
	url1, _ := common.NewURL("consumer://1.1.1.1/com.foo.BarService?methods=getFoo")
	matchWhen = router.(*ConditionRouter).MatchWhen(&url1, inv)
	assert.Equal(t, true, matchWhen)
	url2, _ := common.NewURL("consumer://1.1.1.1/com.foo.BarService?methods=getFoo")
	rule2 := base64.URLEncoding.EncodeToString([]byte("methods=getFoo & host!=1.1.1.1 => host = 1.2.3.4"))
	router2, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule2))
	matchWhen = router2.(*ConditionRouter).MatchWhen(&url2, inv)
	assert.Equal(t, false, matchWhen)
	url3, _ := common.NewURL("consumer://1.1.1.1/com.foo.BarService?methods=getFoo")
	rule3 := base64.URLEncoding.EncodeToString([]byte("methods=getFoo & host=1.1.1.1 => host = 1.2.3.4"))
	router3, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule3))
	matchWhen = router3.(*ConditionRouter).MatchWhen(&url3, inv)
	assert.Equal(t, true, matchWhen)

}

func TestRouteReturnFalse(t *testing.T) {
	url, _ := common.NewURL("")
	localIP, _ := gxnet.GetLocalIP()
	invokers := []protocol.Invoker{NewMockInvoker(url, 1), NewMockInvoker(url, 2), NewMockInvoker(url, 3)}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => false"))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 0, len(fileredInvokers))
}

func TestRouteReturnEmpty(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url, _ := common.NewURL("")
	invokers := []protocol.Invoker{NewMockInvoker(url, 1), NewMockInvoker(url, 2), NewMockInvoker(url, 3)}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => "))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 0, len(fileredInvokers))
}

func TestRouteReturnAll(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	urlString := "dubbo://" + localIP + "/com.foo.BarService"
	dubboURL, _ := common.NewURL(urlString)
	mockInvoker1 := NewMockInvoker(dubboURL, 1)
	mockInvoker2 := NewMockInvoker(dubboURL, 1)
	mockInvoker3 := NewMockInvoker(dubboURL, 1)
	invokers := []protocol.Invoker{mockInvoker1, mockInvoker2, mockInvoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = " + localIP))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, invokers, fileredInvokers)
}

func TestRouteHostFilter(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invoker1 := NewMockInvoker(url1, 1)
	invoker2 := NewMockInvoker(url2, 2)
	invoker3 := NewMockInvoker(url3, 3)
	invokers := []protocol.Invoker{invoker1, invoker2, invoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = " + localIP))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 2, len(fileredInvokers))
	assert.Equal(t, invoker2, fileredInvokers[0])
	assert.Equal(t, invoker3, fileredInvokers[1])
}

func TestRouteEmptyHostFilter(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invoker1 := NewMockInvoker(url1, 1)
	invoker2 := NewMockInvoker(url2, 2)
	invoker3 := NewMockInvoker(url3, 3)
	invokers := []protocol.Invoker{invoker1, invoker2, invoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte(" => " + " host = " + localIP))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 2, len(fileredInvokers))
	assert.Equal(t, invoker2, fileredInvokers[0])
	assert.Equal(t, invoker3, fileredInvokers[1])
}

func TestRouteFalseHostFilter(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invoker1 := NewMockInvoker(url1, 1)
	invoker2 := NewMockInvoker(url2, 2)
	invoker3 := NewMockInvoker(url3, 3)
	invokers := []protocol.Invoker{invoker1, invoker2, invoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("true => " + " host = " + localIP))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 2, len(fileredInvokers))
	assert.Equal(t, invoker2, fileredInvokers[0])
	assert.Equal(t, invoker3, fileredInvokers[1])
}

func TestRoutePlaceholder(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invoker1 := NewMockInvoker(url1, 1)
	invoker2 := NewMockInvoker(url2, 2)
	invoker3 := NewMockInvoker(url3, 3)
	invokers := []protocol.Invoker{invoker1, invoker2, invoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = $host"))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrl(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 2, len(fileredInvokers))
	assert.Equal(t, invoker2, fileredInvokers[0])
	assert.Equal(t, invoker3, fileredInvokers[1])
}

func TestRouteNoForce(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invoker1 := NewMockInvoker(url1, 1)
	invoker2 := NewMockInvoker(url2, 2)
	invoker3 := NewMockInvoker(url3, 3)
	invokers := []protocol.Invoker{invoker1, invoker2, invoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = 1.2.3.4"))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrlWithNoForce(rule))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, invokers, fileredInvokers)
}

func TestRouteForce(t *testing.T) {
	localIP, _ := gxnet.GetLocalIP()
	url1, _ := common.NewURL("dubbo://10.20.3.3:20880/com.foo.BarService")
	url2, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	url3, _ := common.NewURL(fmt.Sprintf("dubbo://%s:20880/com.foo.BarService", localIP))
	invoker1 := NewMockInvoker(url1, 1)
	invoker2 := NewMockInvoker(url2, 2)
	invoker3 := NewMockInvoker(url3, 3)
	invokers := []protocol.Invoker{invoker1, invoker2, invoker3}
	inv := &invocation.RPCInvocation{}
	rule := base64.URLEncoding.EncodeToString([]byte("host = " + localIP + " => " + " host = 1.2.3.4"))
	curl, _ := common.NewURL("consumer://" + localIP + "/com.foo.BarService")
	router, _ := newConditionRouterFactory().NewPriorityRouter(getRouteUrlWithForce(rule, "true"))
	fileredInvokers := router.(*ConditionRouter).Route(invokers, &curl, inv)
	assert.Equal(t, 0, len(fileredInvokers))
}

func TestNewConditionRouterFactory(t *testing.T) {
	factory := newConditionRouterFactory()
	assert.NotNil(t, factory)
}

func TestNewAppRouterFactory(t *testing.T) {
	factory := newAppRouterFactory()
	assert.NotNil(t, factory)
}
