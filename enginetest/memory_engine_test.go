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

package enginetest_test

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/analyzer"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/go-mysql-server/sql/parse"
	"github.com/dolthub/go-mysql-server/sql/plan"
)

// This file is for validating both the engine itself and the in-memory database implementation in the memory package.
// Any engine test that relies on the correct implementation of the in-memory database belongs here. All test logic and
// queries are declared in the exported enginetest package to make them usable by integrators, to validate the engine
// against their own implementation.

type indexBehaviorTestParams struct {
	name              string
	driverInitializer enginetest.IndexDriverInitalizer
	nativeIndexes     bool
}

const testNumPartitions = 5

var numPartitionsVals = []int{
	1,
	testNumPartitions,
}
var indexBehaviors = []*indexBehaviorTestParams{
	{"none", nil, false},
	{"mergableIndexes", mergableIndexDriver, false},
	{"nativeIndexes", nil, true},
	{"nativeAndMergable", mergableIndexDriver, true},
}
var parallelVals = []int{
	1,
	2,
}

// TestQueries tests the given queries on an engine under a variety of circumstances:
// 1) Partitioned tables / non partitioned tables
// 2) Mergeable / unmergeable / native / no indexes
// 3) Parallelism on / off
func TestQueries(t *testing.T) {
	for _, numPartitions := range numPartitionsVals {
		for _, indexBehavior := range indexBehaviors {
			for _, parallelism := range parallelVals {
				if parallelism == 1 && numPartitions == testNumPartitions && indexBehavior.name == "nativeIndexes" {
					// This case is covered by TestQueriesSimple
					continue
				}
				testName := fmt.Sprintf("partitions=%d,indexes=%v,parallelism=%v", numPartitions, indexBehavior.name, parallelism)
				harness := enginetest.NewMemoryHarness(testName, parallelism, numPartitions, indexBehavior.nativeIndexes, indexBehavior.driverInitializer)

				t.Run(testName, func(t *testing.T) {
					enginetest.TestQueries(t, harness)
				})
			}
		}
	}
}

func TestSpatialQueries(t *testing.T) {
	for _, numPartitions := range numPartitionsVals {
		for _, indexBehavior := range indexBehaviors {
			for _, parallelism := range parallelVals {
				if parallelism == 1 && numPartitions == testNumPartitions && indexBehavior.name == "nativeIndexes" {
					// This case is covered by TestQueriesSimple
					continue
				}
				testName := fmt.Sprintf("partitions=%d,indexes=%v,parallelism=%v", numPartitions, indexBehavior.name, parallelism)
				harness := enginetest.NewMemoryHarness(testName, parallelism, numPartitions, indexBehavior.nativeIndexes, indexBehavior.driverInitializer)

				t.Run(testName, func(t *testing.T) {
					enginetest.TestSpatialQueries(t, harness)
				})
			}
		}
	}
}

// TestQueriesPrepared runs the canonical test queries against the gamut of thread, index and partition options
// with prepared statement caching enabled.
func TestQueriesPrepared(t *testing.T) {
	for _, numPartitions := range numPartitionsVals {
		for _, indexBehavior := range indexBehaviors {
			for _, parallelism := range parallelVals {
				testName := fmt.Sprintf("partitions=%d,indexes=%v,parallelism=%v", numPartitions, indexBehavior.name, parallelism)
				harness := enginetest.NewMemoryHarness(testName, parallelism, numPartitions, indexBehavior.nativeIndexes, indexBehavior.driverInitializer)

				t.Run(testName, func(t *testing.T) {
					enginetest.TestQueriesPrepared(t, harness)
				})
			}
		}
	}
}

// TestQueriesSimple runs the canonical test queries against a single threaded index enabled harness.
func TestSpatialQueriesPrepared(t *testing.T) {
	for _, numPartitions := range numPartitionsVals {
		for _, indexBehavior := range indexBehaviors {
			for _, parallelism := range parallelVals {
				testName := fmt.Sprintf("partitions=%d,indexes=%v,parallelism=%v", numPartitions, indexBehavior.name, parallelism)
				harness := enginetest.NewMemoryHarness(testName, parallelism, numPartitions, indexBehavior.nativeIndexes, indexBehavior.driverInitializer)

				t.Run(testName, func(t *testing.T) {
					enginetest.TestSpatialQueriesPrepared(t, harness)
				})
			}
		}
	}
}

// TestQueriesSimple runs the canonical test queries against a single threaded index enabled harness.
func TestSpatialQueriesSimple(t *testing.T) {
	enginetest.TestSpatialQueries(t, enginetest.NewMemoryHarness("simple", 1, testNumPartitions, true, nil))
}

func TestPreparedStaticIndexQuerySimple(t *testing.T) {
	enginetest.TestPreparedStaticIndexQuery(t, enginetest.NewMemoryHarness("simple", 1, testNumPartitions, true, nil))
}

// TestQueriesSimple runs the canonical test queries against a single threaded index enabled harness.
func TestQueriesSimple(t *testing.T) {
	enginetest.TestQueries(t, enginetest.NewMemoryHarness("simple", 1, testNumPartitions, true, nil))
}

// TestQueriesSimple runs the canonical test queries against a single threaded index enabled harness.
func TestJoinQueries(t *testing.T) {
	enginetest.TestJoinQueries(t, enginetest.NewMemoryHarness("simple", 1, testNumPartitions, true, nil))
}

// Convenience test for debugging a single query. Unskip and set to the desired query.
func TestSingleQuery(t *testing.T) {
	t.Skip()

	var test enginetest.QueryTest
	test = enginetest.QueryTest{
		Query: `show create table two_pk`,
		Expected: []sql.Row{
			{1, 2},
		},
	}

	fmt.Sprintf("%v", test)
	harness := enginetest.NewMemoryHarness("", 1, testNumPartitions, true, nil)
	engine := enginetest.NewEngine(t, harness)
	enginetest.CreateIndexes(t, harness, engine)
	engine.Analyzer.Debug = true
	engine.Analyzer.Verbose = true

	enginetest.TestQuery(t, harness, engine, test.Query, test.Expected, nil)
}

// Convenience test for debugging a single query. Unskip and set to the desired query.
func TestSingleQueryPrepared(t *testing.T) {
	t.Skip()

	var test enginetest.QueryTest
	test = enginetest.QueryTest{
		Query: `SELECT ST_SRID(g, 0) from geometry_table order by i`,
		Expected: []sql.Row{
			{sql.Point{X: 1, Y: 2}},
			{sql.Linestring{Points: []sql.Point{{X: 1, Y: 2}, {X: 3, Y: 4}}}},
			{sql.Polygon{Lines: []sql.Linestring{{Points: []sql.Point{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}, {X: 0, Y: 0}}}}}},
			{sql.Point{X: 1, Y: 2}},
			{sql.Linestring{Points: []sql.Point{{X: 1, Y: 2}, {X: 3, Y: 4}}}},
			{sql.Polygon{Lines: []sql.Linestring{{Points: []sql.Point{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}, {X: 0, Y: 0}}}}}},
		},
	}

	fmt.Sprintf("%v", test)
	harness := enginetest.NewMemoryHarness("", 1, testNumPartitions, true, nil)
	//engine := enginetest.NewEngine(t, harness)
	engine := enginetest.NewSpatialEngine(t, harness)
	//enginetest.CreateIndexes(t, harness, engine)
	engine.Analyzer.Debug = true
	engine.Analyzer.Verbose = true

	enginetest.TestPreparedQuery(t, harness, engine, test.Query, test.Expected, nil)
}

// Convenience test for debugging a single query. Unskip and set to the desired query.
func TestSingleScript(t *testing.T) {
	t.Skip()

	var scripts = []enginetest.ScriptTest{
		{
			Name: "ALTER TABLE MULTI ADD/DROP COLUMN",
			SetUpScript: []string{
				"CREATE TABLE test (pk BIGINT PRIMARY KEY, v1 BIGINT NOT NULL DEFAULT 88);",
			},
			Assertions: []enginetest.ScriptTestAssertion{
				{
					Query:    "INSERT INTO test (pk) VALUES (1);",
					Expected: []sql.Row{{sql.NewOkResult(1)}},
				},
				{
					Query:    "ALTER TABLE test DROP COLUMN v1, ADD COLUMN v2 INT NOT NULL DEFAULT 100",
					Expected: []sql.Row{{sql.NewOkResult(0)}},
				},
				{
					Query: "describe test",
					Expected: []sql.Row{
						{"pk", "bigint", "NO", "PRI", "", ""},
						{"v2", "int", "NO", "", "100", ""},
					},
				},
				{
					Query:    "ALTER TABLE TEST MODIFY COLUMN pk BIGINT AUTO_INCREMENT, AUTO_INCREMENT = 100",
					Expected: []sql.Row{{sql.NewOkResult(0)}},
				},
				{
					Query:    "INSERT INTO test (v2) values (11)",
					Expected: []sql.Row{{sql.OkResult{RowsAffected: 1, InsertID: 100}}},
				},
				{
					Query:    "SELECT * from test where pk = 100",
					Expected: []sql.Row{{100, 11}},
				},
				{
					Query:       "ALTER TABLE test DROP COLUMN v2, ADD COLUMN v3 int NOT NULL after v2",
					ExpectedErr: sql.ErrTableColumnNotFound,
				},
				{
					Query: "describe test",
					Expected: []sql.Row{
						{"pk", "bigint", "NO", "PRI", "", "auto_increment"},
						{"v2", "int", "NO", "", "100", ""},
					},
				},
				{
					Query:       "ALTER TABLE test DROP COLUMN v2, RENAME COLUMN v2 to v3",
					ExpectedErr: sql.ErrTableColumnNotFound,
				},
				{
					Query: "describe test",
					Expected: []sql.Row{
						{"pk", "bigint", "NO", "PRI", "", "auto_increment"},
						{"v2", "int", "NO", "", "100", ""},
					},
				},
				{
					Query:       "ALTER TABLE test RENAME COLUMN v2 to v3, DROP COLUMN v2",
					ExpectedErr: sql.ErrTableColumnNotFound,
				},
				{
					Query: "describe test",
					Expected: []sql.Row{
						{"pk", "bigint", "NO", "PRI", "", "auto_increment"},
						{"v2", "int", "NO", "", "100", ""},
					},
				},
				{
					Query:    "ALTER TABLE test ADD COLUMN (v3 int NOT NULL), add column (v4 int), drop column v2, add column (v5 int NOT NULL)",
					Expected: []sql.Row{{sql.NewOkResult(0)}},
				},
				{
					Query: "DESCRIBE test",
					Expected: []sql.Row{
						{"pk", "bigint", "NO", "PRI", "", "auto_increment"},
						{"v3", "int", "NO", "", "", ""},
						{"v4", "int", "YES", "", "", ""},
						{"v5", "int", "NO", "", "", ""},
					},
				},
				{
					Query:    "ALTER TABLE test ADD COLUMN (v6 int not null), RENAME COLUMN v5 TO mycol, DROP COLUMN v4, ADD COLUMN (v7 int);",
					Expected: []sql.Row{{sql.NewOkResult(0)}},
				},
				{
					Query: "describe test",
					Expected: []sql.Row{
						{"pk", "bigint", "NO", "PRI", "", "auto_increment"},
						{"v3", "int", "NO", "", "", ""},
						{"mycol", "int", "NO", "", "", ""},
						{"v6", "int", "NO", "", "", ""},
						{"v7", "int", "YES", "", "", ""},
					},
				},
				// TODO: Does not include tests with column renames and defaults.
			},
		},
	}

	for _, test := range scripts {
		harness := enginetest.NewMemoryHarness("", 1, testNumPartitions, true, nil)
		engine := enginetest.NewEngine(t, harness)
		// engine.Analyzer.Debug = true
		// engine.Analyzer.Verbose = true

		enginetest.TestScriptWithEngine(t, engine, harness, test)
	}
}

func TestUnbuildableIndex(t *testing.T) {
	var scripts = []enginetest.ScriptTest{
		{
			Name: "Failing index builder still returning correct results",
			SetUpScript: []string{
				"CREATE TABLE mytable2 (i BIGINT PRIMARY KEY, s VARCHAR(20))",
				"CREATE UNIQUE INDEX mytable2_s ON mytable2 (s)",
				fmt.Sprintf("CREATE INDEX mytable2_i_s ON mytable2 (i, s) COMMENT '%s'", memory.CommentPreventingIndexBuilding),
				"INSERT INTO mytable2 VALUES (1, 'first row'), (2, 'second row'), (3, 'third row')",
			},
			Assertions: []enginetest.ScriptTestAssertion{
				{
					Query: "SELECT i FROM mytable2 WHERE i IN (SELECT i FROM mytable2) ORDER BY i",
					Expected: []sql.Row{
						{1},
						{2},
						{3},
					},
				},
			},
		},
	}

	for _, test := range scripts {
		harness := enginetest.NewMemoryHarness("", 1, testNumPartitions, true, nil)
		engine := enginetest.NewEngine(t, harness)

		enginetest.TestScriptWithEngine(t, engine, harness, test)
	}
}

func TestBrokenQueries(t *testing.T) {
	enginetest.RunQueryTests(t, enginetest.NewSkippingMemoryHarness(), enginetest.BrokenQueries)
}

func TestTestQueryPlanTODOs(t *testing.T) {
	harness := enginetest.NewSkippingMemoryHarness()
	engine := enginetest.NewEngine(t, harness)
	for _, tt := range enginetest.QueryPlanTODOs {
		t.Run(tt.Query, func(t *testing.T) {
			enginetest.TestQueryPlan(t, enginetest.NewContextWithEngine(harness, engine), engine, harness, tt.Query, tt.ExpectedPlan)
		})
	}
}

func TestVersionedQueries(t *testing.T) {
	for _, numPartitions := range numPartitionsVals {
		for _, indexInit := range indexBehaviors {
			for _, parallelism := range parallelVals {
				testName := fmt.Sprintf("partitions=%d,indexes=%v,parallelism=%v", numPartitions, indexInit.name, parallelism)
				harness := enginetest.NewMemoryHarness(testName, parallelism, numPartitions, indexInit.nativeIndexes, indexInit.driverInitializer)

				t.Run(testName, func(t *testing.T) {
					enginetest.TestVersionedQueries(t, harness)
				})
			}
		}
	}
}

func TestVersionedQueriesPrepared(t *testing.T) {
	for _, numPartitions := range numPartitionsVals {
		for _, indexInit := range indexBehaviors {
			for _, parallelism := range parallelVals {
				testName := fmt.Sprintf("partitions=%d,indexes=%v,parallelism=%v", numPartitions, indexInit.name, parallelism)
				harness := enginetest.NewMemoryHarness(testName, parallelism, numPartitions, indexInit.nativeIndexes, indexInit.driverInitializer)

				t.Run(testName, func(t *testing.T) {
					enginetest.TestVersionedQueriesPrepared(t, harness)
				})
			}
		}
	}
}

// Tests of choosing the correct execution plan independent of result correctness. Mostly useful for confirming that
// the right indexes are being used for joining tables.
func TestQueryPlans(t *testing.T) {
	indexBehaviors := []*indexBehaviorTestParams{
		{"nativeIndexes", nil, true},
		{"nativeAndMergable", mergableIndexDriver, true},
	}

	for _, indexInit := range indexBehaviors {
		t.Run(indexInit.name, func(t *testing.T) {
			harness := enginetest.NewMemoryHarness(indexInit.name, 1, 2, indexInit.nativeIndexes, indexInit.driverInitializer)
			// The IN expression requires mergeable indexes meaning that an unmergeable index returns a different result, so we skip this test
			harness.QueriesToSkip("SELECT a.* FROM mytable a inner join mytable b on (a.i = b.s) WHERE a.i in (1, 2, 3, 4)")
			enginetest.TestQueryPlans(t, harness)
		})
	}
}

func TestIndexQueryPlans(t *testing.T) {
	indexBehaviors := []*indexBehaviorTestParams{
		{"nativeIndexes", nil, true},
		{"nativeAndMergable", mergableIndexDriver, true},
	}

	for _, indexInit := range indexBehaviors {
		t.Run(indexInit.name, func(t *testing.T) {
			harness := enginetest.NewMemoryHarness(indexInit.name, 1, 2, indexInit.nativeIndexes, indexInit.driverInitializer)
			enginetest.TestIndexQueryPlans(t, harness)
		})
	}
}

// This test will write a new set of query plan expected results to a file that you can copy and paste over the existing
// query plan results. Handy when you've made a large change to the analyzer or node formatting, and you want to examine
// how query plans have changed without a lot of manual copying and pasting.
func TestWriteQueryPlans(t *testing.T) {
	//t.Skip()

	harness := enginetest.NewDefaultMemoryHarness()
	engine := enginetest.NewEngine(t, harness)
	enginetest.CreateIndexes(t, harness, engine)

	tmp, err := ioutil.TempDir("", "*")
	if err != nil {
		return
	}

	outputPath := filepath.Join(tmp, "queryPlans.txt")
	f, err := os.Create(outputPath)
	require.NoError(t, err)

	w := bufio.NewWriter(f)
	_, _ = w.WriteString("var PlanTests = []QueryPlanTest{\n")
	for _, tt := range enginetest.PlanTests {
		_, _ = w.WriteString("\t{\n")
		ctx := enginetest.NewContextWithEngine(harness, engine)
		parsed, err := parse.Parse(ctx, tt.Query)
		require.NoError(t, err)

		node, err := engine.Analyzer.Analyze(ctx, parsed, nil)
		require.NoError(t, err)
		planString := extractQueryNode(node).String()

		if strings.Contains(tt.Query, "`") {
			_, _ = w.WriteString(fmt.Sprintf(`Query: "%s",`, tt.Query))
		} else {
			_, _ = w.WriteString(fmt.Sprintf("Query: `%s`,", tt.Query))
		}
		_, _ = w.WriteString("\n")

		_, _ = w.WriteString(`ExpectedPlan: `)
		for i, line := range strings.Split(planString, "\n") {
			if i > 0 {
				_, _ = w.WriteString(" + \n")
			}
			if len(line) > 0 {
				_, _ = w.WriteString(fmt.Sprintf(`"%s\n"`, strings.ReplaceAll(line, `"`, `\"`)))
			} else {
				// final line with comma
				_, _ = w.WriteString("\"\",\n")
			}
		}
		_, _ = w.WriteString("\t},\n")
	}
	_, _ = w.WriteString("}")

	_ = w.Flush()

	t.Logf("Query plans in %s", outputPath)
}

func TestWriteIndexQueryPlans(t *testing.T) {
	t.Skip()

	harness := enginetest.NewDefaultMemoryHarness()
	engine := enginetest.NewEngine(t, harness)

	enginetest.CreateIndexes(t, harness, engine)
	for i, script := range enginetest.ComplexIndexQueries {
		for _, statement := range script.SetUpScript {
			statement = strings.Replace(statement, "test", fmt.Sprintf("t%d", i), -1)
			enginetest.RunQuery(t, engine, harness, statement)
		}
	}

	tmp, err := ioutil.TempDir("", "*")
	if err != nil {
		return
	}

	outputPath := filepath.Join(tmp, "indexQueryPlans.txt")
	f, err := os.Create(outputPath)
	require.NoError(t, err)

	w := bufio.NewWriter(f)
	_, _ = w.WriteString("var IndexPlanTests = []QueryPlanTest{\n")
	for _, tt := range enginetest.IndexPlanTests {
		_, _ = w.WriteString("\t{\n")
		ctx := enginetest.NewContextWithEngine(harness, engine)
		parsed, err := parse.Parse(ctx, tt.Query)
		require.NoError(t, err)

		node, err := engine.Analyzer.Analyze(ctx, parsed, nil)
		require.NoError(t, err)
		planString := extractQueryNode(node).String()

		if strings.Contains(tt.Query, "`") {
			_, _ = w.WriteString(fmt.Sprintf(`Query: "%s",`, tt.Query))
		} else {
			_, _ = w.WriteString(fmt.Sprintf("Query: `%s`,", tt.Query))
		}
		_, _ = w.WriteString("\n")

		_, _ = w.WriteString(`ExpectedPlan: `)
		for i, line := range strings.Split(planString, "\n") {
			if i > 0 {
				_, _ = w.WriteString(" + \n")
			}
			if len(line) > 0 {
				_, _ = w.WriteString(fmt.Sprintf(`"%s\n"`, strings.ReplaceAll(line, `"`, `\"`)))
			} else {
				// final line with comma
				_, _ = w.WriteString("\"\",\n")
			}
		}
		_, _ = w.WriteString("\t},\n")
	}
	_, _ = w.WriteString("}")

	_ = w.Flush()

	t.Logf("Query plans in %s", outputPath)
}

func extractQueryNode(node sql.Node) sql.Node {
	switch node := node.(type) {
	case *plan.QueryProcess:
		return extractQueryNode(node.Child())
	case *analyzer.Releaser:
		return extractQueryNode(node.Child)
	default:
		return node
	}
}

func TestQueryErrors(t *testing.T) {
	enginetest.TestQueryErrors(t, enginetest.NewDefaultMemoryHarness())
}

func TestInfoSchema(t *testing.T) {
	enginetest.TestInfoSchema(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestInfoSchemaPrepared(t *testing.T) {
	enginetest.TestInfoSchemaPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestReadOnlyDatabases(t *testing.T) {
	enginetest.TestReadOnlyDatabases(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestColumnAliases(t *testing.T) {
	enginetest.TestColumnAliases(t, enginetest.NewDefaultMemoryHarness())
}

func TestOrderByGroupBy(t *testing.T) {
	enginetest.TestOrderByGroupBy(t, enginetest.NewDefaultMemoryHarness())
}

func TestAmbiguousColumnResolution(t *testing.T) {
	enginetest.TestAmbiguousColumnResolution(t, enginetest.NewDefaultMemoryHarness())
}

func TestInsertInto(t *testing.T) {
	enginetest.TestInsertInto(t, enginetest.NewDefaultMemoryHarness())
}

func TestInsertIgnoreInto(t *testing.T) {
	enginetest.TestInsertIgnoreInto(t, enginetest.NewDefaultMemoryHarness())
}

func TestInsertIntoErrors(t *testing.T) {
	enginetest.TestInsertIntoErrors(t, enginetest.NewDefaultMemoryHarness())
}

func TestBrokenInsertScripts(t *testing.T) {
	enginetest.TestBrokenInsertScripts(t, enginetest.NewSkippingMemoryHarness())
}

func TestSpatialInsertInto(t *testing.T) {
	enginetest.TestSpatialInsertInto(t, enginetest.NewDefaultMemoryHarness())
}

func TestLoadData(t *testing.T) {
	enginetest.TestLoadData(t, enginetest.NewDefaultMemoryHarness())
}

func TestLoadDataErrors(t *testing.T) {
	enginetest.TestLoadDataErrors(t, enginetest.NewDefaultMemoryHarness())
}

func TestLoadDataFailing(t *testing.T) {
	enginetest.TestLoadDataFailing(t, enginetest.NewDefaultMemoryHarness())
}

func TestReplaceInto(t *testing.T) {
	enginetest.TestReplaceInto(t, enginetest.NewDefaultMemoryHarness())
}

func TestReplaceIntoErrors(t *testing.T) {
	enginetest.TestReplaceIntoErrors(t, enginetest.NewDefaultMemoryHarness())
}

func TestUpdate(t *testing.T) {
	enginetest.TestUpdate(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestUpdateErrors(t *testing.T) {
	enginetest.TestUpdateErrors(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestSpatialUpdate(t *testing.T) {
	enginetest.TestSpatialUpdate(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestDeleteQueriesPrepared(t *testing.T) {
	enginetest.TestDeleteQueriesPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestInsertQueriesPrepared(t *testing.T) {
	enginetest.TestInsertQueriesPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestUpdateQueriesPrepared(t *testing.T) {
	enginetest.TestUpdateQueriesPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestReplaceQueriesPrepared(t *testing.T) {
	enginetest.TestReplaceQueriesPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestDeleteFromErrors(t *testing.T) {
	enginetest.TestDeleteErrors(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestSpatialDeleteFrom(t *testing.T) {
	enginetest.TestSpatialDelete(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestTruncate(t *testing.T) {
	enginetest.TestTruncate(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestDeleteFrom(t *testing.T) {
	enginetest.TestDelete(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestScripts(t *testing.T) {
	enginetest.TestScripts(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestSpatialScripts(t *testing.T) {
	enginetest.TestSpatialScripts(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestLoadDataPrepared(t *testing.T) {
	enginetest.TestLoadDataPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestScriptsPrepared(t *testing.T) {
	//TODO: when foreign keys are implemented in the memory table, we can do the following test
	for i := len(enginetest.ScriptTests) - 1; i >= 0; i-- {
		if enginetest.ScriptTests[i].Name == "failed statements data validation for DELETE, REPLACE" {
			enginetest.ScriptTests = append(enginetest.ScriptTests[:i], enginetest.ScriptTests[i+1:]...)
		}
	}
	enginetest.TestScriptsPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestInsertScriptsPrepared(t *testing.T) {
	enginetest.TestInsertScriptsPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestComplexIndexQueriesPrepared(t *testing.T) {
	enginetest.TestComplexIndexQueriesPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestJsonScriptsPrepared(t *testing.T) {
	enginetest.TestJsonScriptsPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestCreateCheckConstraintsScriptsPrepared(t *testing.T) {
	enginetest.TestCreateCheckConstraintsScriptsPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestInsertIgnoreScriptsPrepared(t *testing.T) {
	enginetest.TestInsertIgnoreScriptsPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestInsertErrorScriptsPrepared(t *testing.T) {
	enginetest.TestInsertErrorScriptsPrepared(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestScriptQueryPlan(t *testing.T) {
	enginetest.TestScriptQueryPlan(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestUserPrivileges(t *testing.T) {
	enginetest.TestUserPrivileges(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestUserAuthentication(t *testing.T) {
	enginetest.TestUserAuthentication(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestPrivilegePersistence(t *testing.T) {
	enginetest.TestPrivilegePersistence(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestComplexIndexQueries(t *testing.T) {
	harness := enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver)
	enginetest.TestComplexIndexQueries(t, harness)
}

func TestTriggers(t *testing.T) {
	enginetest.TestTriggers(t, enginetest.NewDefaultMemoryHarness())
}

func TestShowTriggers(t *testing.T) {
	enginetest.TestShowTriggers(t, enginetest.NewDefaultMemoryHarness())
}

func TestBrokenTriggers(t *testing.T) {
	h := enginetest.NewSkippingMemoryHarness()
	for _, script := range enginetest.BrokenTriggerQueries {
		enginetest.TestScript(t, h, script)
	}
}

func TestStoredProcedures(t *testing.T) {
	enginetest.TestStoredProcedures(t, enginetest.NewDefaultMemoryHarness())
}

func TestExternalProcedures(t *testing.T) {
	harness := enginetest.NewExternalStoredProcedureMemoryHarness()
	for _, script := range enginetest.ExternalProcedureTests {
		enginetest.TestScript(t, harness, script)
	}
}

func TestTriggersErrors(t *testing.T) {
	enginetest.TestTriggerErrors(t, enginetest.NewDefaultMemoryHarness())
}

func TestCreateTable(t *testing.T) {
	enginetest.TestCreateTable(t, enginetest.NewDefaultMemoryHarness())
}

func TestDropTable(t *testing.T) {
	enginetest.TestDropTable(t, enginetest.NewDefaultMemoryHarness())
}

func TestRenameTable(t *testing.T) {
	enginetest.TestRenameTable(t, enginetest.NewDefaultMemoryHarness())
}

func TestRenameColumn(t *testing.T) {
	enginetest.TestRenameColumn(t, enginetest.NewDefaultMemoryHarness())
}

func TestAddColumn(t *testing.T) {
	enginetest.TestAddColumn(t, enginetest.NewDefaultMemoryHarness())
}

func TestModifyColumn(t *testing.T) {
	enginetest.TestModifyColumn(t, enginetest.NewDefaultMemoryHarness())
}

func TestDropColumn(t *testing.T) {
	enginetest.TestDropColumn(t, enginetest.NewDefaultMemoryHarness())
}

func TestCreateDatabase(t *testing.T) {
	enginetest.TestCreateDatabase(t, enginetest.NewDefaultMemoryHarness())
}

func TestPkOrdinalsDDL(t *testing.T) {
	enginetest.TestPkOrdinalsDDL(t, enginetest.NewDefaultMemoryHarness())
}

func TestPkOrdinalsDML(t *testing.T) {
	enginetest.TestPkOrdinalsDML(t, enginetest.NewDefaultMemoryHarness())
}

func TestDropDatabase(t *testing.T) {
	enginetest.TestDropDatabase(t, enginetest.NewDefaultMemoryHarness())
}

func TestCreateForeignKeys(t *testing.T) {
	enginetest.TestCreateForeignKeys(t, enginetest.NewDefaultMemoryHarness())
}

func TestDropForeignKeys(t *testing.T) {
	enginetest.TestDropForeignKeys(t, enginetest.NewDefaultMemoryHarness())
}

func TestForeignKeys(t *testing.T) {
	enginetest.TestForeignKeys(t, enginetest.NewDefaultMemoryHarness())
}

func TestCreateCheckConstraints(t *testing.T) {
	enginetest.TestCreateCheckConstraints(t, enginetest.NewDefaultMemoryHarness())
}

func TestChecksOnInsert(t *testing.T) {
	enginetest.TestChecksOnInsert(t, enginetest.NewDefaultMemoryHarness())
}

func TestChecksOnUpdate(t *testing.T) {
	enginetest.TestChecksOnUpdate(t, enginetest.NewDefaultMemoryHarness())
}

func TestDisallowedCheckConstraints(t *testing.T) {
	enginetest.TestDisallowedCheckConstraints(t, enginetest.NewDefaultMemoryHarness())
}

func TestDropCheckConstraints(t *testing.T) {
	enginetest.TestDropCheckConstraints(t, enginetest.NewDefaultMemoryHarness())
}

func TestDropConstraints(t *testing.T) {
	enginetest.TestDropConstraints(t, enginetest.NewDefaultMemoryHarness())
}

func TestExplode(t *testing.T) {
	enginetest.TestExplode(t, enginetest.NewDefaultMemoryHarness())
}

func TestExplodePrepared(t *testing.T) {
	enginetest.TestExplodePrepared(t, enginetest.NewDefaultMemoryHarness())
}

func TestReadOnly(t *testing.T) {
	enginetest.TestReadOnly(t, enginetest.NewDefaultMemoryHarness())
}

func TestViews(t *testing.T) {
	enginetest.TestViews(t, enginetest.NewDefaultMemoryHarness())
}

func TestViewsPrepared(t *testing.T) {
	enginetest.TestViewsPrepared(t, enginetest.NewDefaultMemoryHarness())
}

func TestVersionedViews(t *testing.T) {
	enginetest.TestVersionedViews(t, enginetest.NewDefaultMemoryHarness())
}

func TestVersionedViewsPrepared(t *testing.T) {
	t.Skip()
	enginetest.TestVersionedViewsPrepared(t, enginetest.NewDefaultMemoryHarness())
}

func TestNaturalJoin(t *testing.T) {
	enginetest.TestNaturalJoin(t, enginetest.NewDefaultMemoryHarness())
}

func TestWindowFunctions(t *testing.T) {
	enginetest.TestWindowFunctions(t, enginetest.NewDefaultMemoryHarness())
}

func TestWindowRowFrames(t *testing.T) {
	enginetest.TestWindowRowFrames(t, enginetest.NewDefaultMemoryHarness())
}

func TestWindowRangeFrames(t *testing.T) {
	enginetest.TestWindowRangeFrames(t, enginetest.NewDefaultMemoryHarness())
}

func TestNamedWindows(t *testing.T) {
	enginetest.TestNamedWindows(t, enginetest.NewDefaultMemoryHarness())
}

func TestNaturalJoinEqual(t *testing.T) {
	enginetest.TestNaturalJoinEqual(t, enginetest.NewDefaultMemoryHarness())
}

func TestNaturalJoinDisjoint(t *testing.T) {
	enginetest.TestNaturalJoinDisjoint(t, enginetest.NewDefaultMemoryHarness())
}

func TestInnerNestedInNaturalJoins(t *testing.T) {
	enginetest.TestInnerNestedInNaturalJoins(t, enginetest.NewDefaultMemoryHarness())
}

func TestColumnDefaults(t *testing.T) {
	enginetest.TestColumnDefaults(t, enginetest.NewDefaultMemoryHarness())
}

func TestAlterTable(t *testing.T) {
	enginetest.TestAlterTable(t, enginetest.NewDefaultMemoryHarness())
}

func TestDateParse(t *testing.T) {
	enginetest.TestDateParse(t, enginetest.NewDefaultMemoryHarness())
}

func TestJsonScripts(t *testing.T) {
	enginetest.TestJsonScripts(t, enginetest.NewDefaultMemoryHarness())
}

func TestShowTableStatus(t *testing.T) {
	enginetest.TestShowTableStatus(t, enginetest.NewDefaultMemoryHarness())
}

func TestShowTableStatusPrepared(t *testing.T) {
	enginetest.TestShowTableStatusPrepared(t, enginetest.NewDefaultMemoryHarness())
}

func TestAddDropPks(t *testing.T) {
	enginetest.TestAddDropPks(t, enginetest.NewDefaultMemoryHarness())
}

func TestNullRanges(t *testing.T) {
	enginetest.TestNullRanges(t, enginetest.NewDefaultMemoryHarness())
}

func TestPersist(t *testing.T) {
	newSess := func(ctx *sql.Context) sql.PersistableSession {
		persistedGlobals := memory.GlobalsMap{}
		persistedSess := memory.NewInMemoryPersistedSession(ctx.Session, persistedGlobals)
		return persistedSess
	}
	enginetest.TestPersist(t, enginetest.NewDefaultMemoryHarness(), newSess)
}

func TestPrepared(t *testing.T) {
	enginetest.TestPrepared(t, enginetest.NewDefaultMemoryHarness())
}

func TestPreparedInsert(t *testing.T) {
	enginetest.TestPreparedInsert(t, enginetest.NewMemoryHarness("default", 1, testNumPartitions, true, mergableIndexDriver))
}

func TestKeylessUniqueIndex(t *testing.T) {
	// TODO: GMS does not support unique indexes for keyless tables.
	t.Skip()
	enginetest.TestKeylessUniqueIndex(t, enginetest.NewDefaultMemoryHarness())
}

func mergableIndexDriver(dbs []sql.Database) sql.IndexDriver {
	return memory.NewIndexDriver("mydb", map[string][]sql.DriverIndex{
		"mytable": {
			newMergableIndex(dbs, "mytable",
				expression.NewGetFieldWithTable(0, sql.Int64, "mytable", "i", false)),
			newMergableIndex(dbs, "mytable",
				expression.NewGetFieldWithTable(1, sql.Text, "mytable", "s", false)),
			newMergableIndex(dbs, "mytable",
				expression.NewGetFieldWithTable(0, sql.Int64, "mytable", "i", false),
				expression.NewGetFieldWithTable(1, sql.Text, "mytable", "s", false)),
		},
		"othertable": {
			newMergableIndex(dbs, "othertable",
				expression.NewGetFieldWithTable(0, sql.Text, "othertable", "s2", false)),
			newMergableIndex(dbs, "othertable",
				expression.NewGetFieldWithTable(1, sql.Text, "othertable", "i2", false)),
			newMergableIndex(dbs, "othertable",
				expression.NewGetFieldWithTable(0, sql.Text, "othertable", "s2", false),
				expression.NewGetFieldWithTable(1, sql.Text, "othertable", "i2", false)),
		},
		"bigtable": {
			newMergableIndex(dbs, "bigtable",
				expression.NewGetFieldWithTable(0, sql.Text, "bigtable", "t", false)),
		},
		"floattable": {
			newMergableIndex(dbs, "floattable",
				expression.NewGetFieldWithTable(2, sql.Text, "floattable", "f64", false)),
		},
		"niltable": {
			newMergableIndex(dbs, "niltable",
				expression.NewGetFieldWithTable(0, sql.Int64, "niltable", "i", false)),
			newMergableIndex(dbs, "niltable",
				expression.NewGetFieldWithTable(1, sql.Int64, "niltable", "i2", true)),
		},
		"one_pk": {
			newMergableIndex(dbs, "one_pk",
				expression.NewGetFieldWithTable(0, sql.Int8, "one_pk", "pk", false)),
		},
		"two_pk": {
			newMergableIndex(dbs, "two_pk",
				expression.NewGetFieldWithTable(0, sql.Int8, "two_pk", "pk1", false),
				expression.NewGetFieldWithTable(1, sql.Int8, "two_pk", "pk2", false),
			),
		},
	})
}

func newMergableIndex(dbs []sql.Database, tableName string, exprs ...sql.Expression) *memory.Index {
	db, table := findTable(dbs, tableName)
	if db == nil {
		return nil
	}
	return &memory.Index{
		DB:         db.Name(),
		DriverName: memory.IndexDriverId,
		TableName:  tableName,
		Tbl:        table.(*memory.Table),
		Exprs:      exprs,
	}
}

func findTable(dbs []sql.Database, tableName string) (sql.Database, sql.Table) {
	for _, db := range dbs {
		names, err := db.GetTableNames(sql.NewEmptyContext())
		if err != nil {
			panic(err)
		}
		for _, name := range names {
			if name == tableName {
				table, _, _ := db.GetTableInsensitive(sql.NewEmptyContext(), name)
				return db, table
			}
		}
	}
	return nil, nil
}
