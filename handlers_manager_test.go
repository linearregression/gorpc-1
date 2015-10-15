package gorpc

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"

	test_handler1 "github.com/sergei-svistunov/gorpc/test/handler1"
)

// Suite definition
type HandlersManagerSuite struct {
	suite.Suite
	hm *HandlersManager
}

func (s *HandlersManagerSuite) SetupTest() {
	s.hm = NewHandlersManager("github.com/sergei-svistunov/gorpc", HandlersManagerCallbacks{})

	s.NoError(s.hm.RegisterHandler(test_handler1.NewHandler()))
}

func TestRunSuite(t *testing.T) {
	suite.Run(t, new(HandlersManagerSuite))
}

// Tests
func (s *HandlersManagerSuite) TestHandlerManager_CheckHandlersPaths() {
	s.Equal([]string{"/test/handler1"}, s.hm.GetHandlersPaths())
}

func (s *HandlersManagerSuite) TestHandlerManager_FindExistsHandler() {
	s.NotNil(s.hm.FindHandler("/test/handler1", 1))
}

func (s *HandlersManagerSuite) TestHandlerManager_CheckHandler1Struct() {
	hv1 := s.hm.FindHandler("/test/handler1", 1)
	hv2 := s.hm.FindHandler("/test/handler1", 2)

	s.Equal("v1", hv1.Version)
	s.True(hv1.UseCache)
	s.Equal([]handlerParameter{
		handlerParameter{
			Name:        "ReqInt",
			Description: "Required integer argument",
			Key:         "req_int",
			IsRequired:  true,
			RawType:     hv1.Request.Fields[0].RawType,
			getMethod:   hv1.Request.Fields[0].getMethod,
			structField: hv1.Request.Fields[0].structField,
		},
		handlerParameter{
			Name:        "Int",
			Description: "Unrequired integer argument",
			Key:         "int",
			IsRequired:  false,
			RawType:     hv1.Request.Fields[1].RawType,
			getMethod:   hv1.Request.Fields[1].getMethod,
			structField: hv1.Request.Fields[1].structField,
		},
	}, hv1.Request.Fields)

	s.Equal("v2", hv2.Version)
	s.False(hv2.UseCache)
	s.Equal([]HandlerError{
		HandlerError{
			UserMessage: "Error 1 description",
			Err:         hv2.Errors[0].Err,
			Code:        "ERROR_TYPE1",
		},
		HandlerError{
			UserMessage: "Error 2 description",
			Err:         hv2.Errors[1].Err,
			Code:        "ERROR_TYPE2",
		},
		HandlerError{
			UserMessage: "Error 3 description",
			Err:         hv2.Errors[2].Err,
			Code:        "ERROR_TYPE3",
		},
	}, hv2.Errors)
}

func (s *HandlersManagerSuite) TestHandlerManager_CallHandler1V1_ReturnResult() {
	pg := &ParametersGetter{
		map[string][]string{
			"req_int": []string{"123"},
		},
	}

	hanlerVersion := s.hm.FindHandler("/test/handler1", 1)
	if hanlerVersion == nil {
		s.NotNil(hanlerVersion)
	}

	params, err := s.hm.PrepareParameters(context.TODO(), hanlerVersion, pg)
	s.NoError(err)

	res, err := s.hm.CallHandler(context.TODO(), hanlerVersion, params)
	s.NoError(err)
	s.Equal(&test_handler1.V1Res{"Test", 123}, res)
}

func (s *HandlersManagerSuite) TestHandlerManager_PrepareParametersWithError() {
	pg := &ParametersGetter{
		map[string][]string{},
	}

	hanlerVersion := s.hm.FindHandler("/test/handler1", 1)
	if hanlerVersion == nil {
		s.NotNil(hanlerVersion)
	}

	_, err := s.hm.PrepareParameters(context.TODO(), hanlerVersion, pg)
	s.Error(err)
}
