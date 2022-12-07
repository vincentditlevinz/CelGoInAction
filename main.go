package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/golang/glog"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"log"
	"net/http"
)

type Variable struct {
	Value any    `json:"value" binding:"required"`
	Type  string `json:"type" binding:"required"`
}

type Input struct {
	Expression string              `json:"expression" binding:"required"`
	Data       map[string]Variable `json:"data" binding:"required"`
}

type Pair[K, V any] struct {
	First  K
	Second V
}

func Entries[M ~map[K]V, K comparable, V any](m M) []Pair[K, V] {
	entries := make([]Pair[K, V], 0)
	for k, v := range m {
		entries = append(entries, Pair[K, V]{First: k, Second: v})
	}
	return entries
}

func Map[T, U any](data []T, f func(T) U) []U {
	res := make([]U, 0, len(data))
	for _, e := range data {
		res = append(res, f(e))
	}
	return res
}

func MapToInterface(input map[string]Variable) map[string]interface{} {
	output := make(map[string]interface{}, len(input))
	for k, v := range input {
		output[k] = v.Value
	}
	return output
}

func CelType(rawType string) *cel.Type {
	if rawType == "String" {
		return cel.StringType
	} else if rawType == "Int" {
		return cel.IntType
	} else {
		return cel.AnyType
	}
}

func evaluate(expression string, input map[string]Variable) ref.Val {
	varsDescriptor := Map(Entries(input), func(variable Pair[string, Variable]) cel.EnvOption {
		return cel.Variable(variable.First, CelType(variable.Second.Type))
	})

	env, err := cel.NewEnv(varsDescriptor...)
	if err != nil {
		glog.Exitf("env error: %v", err)
	}

	ast, issues := env.Compile(expression)

	if issues != nil && issues.Err() != nil {
		log.Fatalf("type-check error: %s", issues.Err())
	}
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval))
	if err != nil {
		log.Fatalf("program construction error: %s", err)
	}

	out, detail, err := prg.Eval(MapToInterface(input))

	if err != nil {
		log.Fatalf("program evaluation error: %s", err)
	}

	if detail != nil {
		fmt.Printf("Detail: %v", detail)
		fmt.Println()
	}
	return out
}

func doEvaluate(c *gin.Context) {
	input := Input{}
	if err := c.ShouldBindBodyWith(&input, binding.JSON); err == nil {
		result := evaluate(input.Expression, input.Data)
		c.IndentedJSON(http.StatusOK, result)
	} else {
		log.Fatalf("Error reading data for /evaluate endpoint: %s", err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

func main() {
	router := gin.Default()
	router.GET("/evaluate", doEvaluate)

	router.Run("localhost:8080")
}
