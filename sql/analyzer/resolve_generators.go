package analyzer

import (
	"gopkg.in/src-d/go-errors.v1"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/go-mysql-server/sql/expression/function"
	"github.com/dolthub/go-mysql-server/sql/plan"
)

var (
	errMultipleGenerators = errors.NewKind("there can't be more than 1 instance of EXPLODE in a SELECT")
	errExplodeNotArray    = errors.NewKind("argument of type %q given to EXPLODE, expecting array")
)

func resolveGenerators(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope) (sql.Node, error) {
	return plan.TransformUp(n, func(n sql.Node) (sql.Node, error) {
		p, ok := n.(*plan.Project)
		if !ok {
			return n, nil
		}

		projection := p.Projections

		g, err := findGenerator(projection)
		if err != nil {
			return nil, err
		}

		// There might be no generator in the project, in that case we don't
		// have to do anything.
		if g == nil {
			return n, nil
		}

		projection[g.idx] = g.expr

		var name string
		if n, ok := g.expr.(sql.Nameable); ok {
			name = n.Name()
		} else {
			name = g.expr.String()
		}

		return plan.NewGenerate(
			plan.NewProject(projection, p.Child),
			expression.NewGetField(g.idx, g.expr.Type(), name, g.expr.IsNullable()),
		), nil
	})
}

type generator struct {
	idx  int
	expr sql.Expression
}

// findGenerator will find in the given projection a generator column. If there
// is no generator, it will return nil.
// If there are is than one generator or the argument to explode is not an
// array it will fail.
// All occurrences of Explode will be replaced with Generate.
func findGenerator(exprs []sql.Expression) (*generator, error) {
	var g = &generator{idx: -1}
	for i, e := range exprs {
		var found bool
		switch e := e.(type) {
		case *function.Explode:
			found = true
			g.expr = function.NewGenerate(e.Child)
		case *expression.Alias:
			if exp, ok := e.Child.(*function.Explode); ok {
				found = true
				g.expr = expression.NewAlias(e.Name(), function.NewGenerate(exp.Child))
			}
		}

		if found {
			if g.idx >= 0 {
				return nil, errMultipleGenerators.New()
			}
			g.idx = i

			if !sql.IsArray(g.expr.Type()) {
				return nil, errExplodeNotArray.New(g.expr.Type())
			}
		}
	}

	if g.expr == nil {
		return nil, nil
	}

	return g, nil
}
