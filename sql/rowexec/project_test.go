// Copyright 2020-2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rowexec

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/go-mysql-server/sql/plan"
	"github.com/dolthub/go-mysql-server/sql/types"
)

func TestProject(t *testing.T) {
	require := require.New(t)
	ctx := sql.NewEmptyContext()
	childSchema := sql.NewPrimaryKeySchema(sql.Schema{
		{Name: "col1", Type: types.Text, Nullable: true},
		{Name: "col2", Type: types.Text, Nullable: true},
	})
	child := memory.NewTable("test", childSchema, nil)
	child.Insert(sql.NewEmptyContext(), sql.NewRow("col1_1", "col2_1"))
	child.Insert(sql.NewEmptyContext(), sql.NewRow("col1_2", "col2_2"))
	p := plan.NewProject(
		[]sql.Expression{expression.NewGetField(1, types.Text, "col2", true)},
		plan.NewResolvedTable(child, nil, nil),
	)
	require.Equal(1, len(p.Children()))
	schema := sql.NewPrimaryKeySchema(sql.Schema{
		{Name: "col2", Type: types.Text, Nullable: true},
	})
	require.Equal(schema.Schema, p.Schema())
	iter, err := DefaultBuilder.Build(ctx, p, nil)
	require.NoError(err)
	require.NotNil(iter)
	row, err := iter.Next(ctx)
	require.NoError(err)
	require.NotNil(row)
	require.Equal(1, len(row))
	require.Equal("col2_1", row[0])
	row, err = iter.Next(ctx)
	require.NoError(err)
	require.NotNil(row)
	require.Equal(1, len(row))
	require.Equal("col2_2", row[0])
	row, err = iter.Next(ctx)
	require.Equal(io.EOF, err)
	require.Nil(row)

	p = plan.NewProject(nil, plan.NewResolvedTable(child, nil, nil))
	require.Equal(0, len(p.Schema()))

	p = plan.NewProject([]sql.Expression{
		expression.NewAlias("foo", expression.NewGetField(1, types.Text, "col2", true)),
	}, plan.NewResolvedTable(child, nil, nil))
	schema = sql.NewPrimaryKeySchema(sql.Schema{
		{Name: "foo", Type: types.Text, Nullable: true},
	})
	require.Equal(schema.Schema, p.Schema())
}

func BenchmarkProject(b *testing.B) {
	require := require.New(b)
	ctx := sql.NewEmptyContext()

	for i := 0; i < b.N; i++ {
		d := plan.NewProject([]sql.Expression{
			expression.NewGetField(0, types.Text, "strfield", true),
			expression.NewGetField(1, types.Float64, "floatfield", true),
			expression.NewGetField(2, types.Boolean, "boolfield", false),
			expression.NewGetField(3, types.Int32, "intfield", false),
			expression.NewGetField(4, types.Int64, "bigintfield", false),
			expression.NewGetField(5, types.Blob, "blobfield", false),
		}, plan.NewResolvedTable(benchtable, nil, nil))

		iter, err := DefaultBuilder.Build(ctx, d, nil)
		require.NoError(err)
		require.NotNil(iter)

		for {
			_, err := iter.Next(ctx)
			if err == io.EOF {
				break
			}

			require.NoError(err)
		}
	}
}
