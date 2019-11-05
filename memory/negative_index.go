package memory

import (
	"github.com/src-d/go-mysql-server/sql"
	"github.com/src-d/go-mysql-server/sql/expression"
)

type NegateIndexLookup struct {
	Lookup MergeableLookup
	Index ExpressionsIndex
}

var _ memoryIndexLookup = (*NegateIndexLookup)(nil)

func (l *NegateIndexLookup) ID() string { return "not " + l.Lookup.ID() }

func (l *NegateIndexLookup) Values(p sql.Partition) (sql.IndexValueIter, error) {
	return &dummyIndexValueIter{
		tbl:       l.Index.MemTable(),
		partition: p,
		matchExpressions: func() []sql.Expression {
			return []sql.Expression { l.EvalExpression() }
		}}, nil
}

func (l *NegateIndexLookup) EvalExpression() sql.Expression {
	return expression.NewNot(l.Lookup.(memoryIndexLookup).EvalExpression())
}

func (l *NegateIndexLookup) Indexes() []string {
	return []string{l.ID()}
}

func (*NegateIndexLookup) IsMergeable(sql.IndexLookup) bool {
	return true
}

func (l *NegateIndexLookup) Union(lookups ...sql.IndexLookup) sql.IndexLookup {
	return union(l.Index, l, lookups...)
}

func (*NegateIndexLookup) Difference(...sql.IndexLookup) sql.IndexLookup {
	panic("negateIndexLookup.Difference is not implemented")
}

func (l *NegateIndexLookup) Intersection(indexes ...sql.IndexLookup) sql.IndexLookup {
	return intersection(l.Index, l, indexes...)
}