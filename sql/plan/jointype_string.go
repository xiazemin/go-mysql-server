// Code generated by "stringer -type=JoinType -linecomment"; DO NOT EDIT.

package plan

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[JoinTypeUnknown-0]
	_ = x[JoinTypeCross-1]
	_ = x[JoinTypeInner-2]
	_ = x[JoinTypeSemi-3]
	_ = x[JoinTypeAnti-4]
	_ = x[JoinTypeLeft-5]
	_ = x[JoinTypeFullOuter-6]
	_ = x[JoinTypeGroupBy-7]
	_ = x[JoinTypeRight-8]
}

const _JoinType_name = "UnknownJoinCrossJoinInnerJoinSemiJoinAntiJoinLeftJoinFullOuterJoinGroupByJoinRightJoin"

var _JoinType_index = [...]uint8{0, 11, 20, 29, 37, 45, 53, 66, 77, 86}

func (i JoinType) String() string {
	if i >= JoinType(len(_JoinType_index)-1) {
		return "JoinType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _JoinType_name[_JoinType_index[i]:_JoinType_index[i+1]]
}
