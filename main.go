package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"log"
	"net/http"
	"time"
)

type Input struct {
	Expression string         `json:"expression" binding:"required"`
	Data       map[string]any `json:"data" binding:"required"`
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

func MapToInterface(input map[string]any) map[string]interface{} {
	output := make(map[string]interface{}, len(input))
	for k, v := range input {
		output[k] = v
	}
	return output
}

func evaluate(expression string, input map[string]any) (ref.Val, error) {
	varsDescriptor := Map(Entries(input), func(variable Pair[string, any]) cel.EnvOption {
		return cel.Variable(variable.First, cel.AnyType)
	})

	// Add now as a variable
	varsDescriptor = append(varsDescriptor, cel.Declarations(decls.NewVar("now", decls.Timestamp)))
	env, err := cel.NewEnv(varsDescriptor...)
	if err != nil {
		return nil, fmt.Errorf("env error: %v", err)
	}

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("type-check error: %v", issues.Err())
	}

	prg, err := env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval))
	if err != nil {
		return nil, fmt.Errorf("program construction error: %v", err)
	}

	// Add now as an input
	input["now"] = types.Timestamp{Time: time.Now()}

	out, detail, err := prg.Eval(MapToInterface(input))

	if err != nil {
		return nil, fmt.Errorf("program evaluation error: %v", err)
	}
	if detail != nil {
		fmt.Printf("Detail: %v", detail)
		fmt.Println()
	}
	return out, nil
}

func doEvaluate(c *gin.Context) {
	input := Input{}
	if err := c.ShouldBindBodyWith(&input, binding.JSON); err == nil {
		result, evaluationError := evaluate(input.Expression, input.Data)
		if evaluationError != nil {
			log.Printf("Evaluation error: %v", evaluationError)
			c.JSON(http.StatusBadRequest, gin.H{"error": evaluationError.Error()})
		} else {
			c.IndentedJSON(http.StatusOK, result)
		}
	} else {
		abortError := c.AbortWithError(http.StatusBadRequest, err)
		if abortError != nil {
			log.Printf("Abort error: %v", abortError)

		}
	}
}

func main() {
	router := gin.Default()
	router.GET("/evaluate", doEvaluate)
	err := router.Run("localhost:8080")
	if err != nil {
		return
	}
}
