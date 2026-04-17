## ADDED Requirements

### Requirement: evaluateCondition supports extended operators

evaluateCondition SHALL support the following new operators in addition to existing ones: "in", "not_in", "is_empty", "is_not_empty", "between", and "matches". An unrecognized operator SHALL return an error.

#### Scenario: in operator matches value in array
- **WHEN** a gateway condition has operator="in" and value=["high","critical"]
- **WHEN** the field value is "high"
- **THEN** the condition evaluates to true

#### Scenario: not_in operator excludes value
- **WHEN** a gateway condition has operator="not_in" and value=["low","medium"]
- **WHEN** the field value is "high"
- **THEN** the condition evaluates to true

### Requirement: "in" operator matches any element in value array

The "in" operator SHALL evaluate to true when the field value exactly matches any element in the condition value array. The condition value MUST be an array. If the field value does not match any element, the condition evaluates to false.

### Requirement: "not_in" operator excludes all elements in value array

The "not_in" operator SHALL evaluate to true when the field value does not exactly match any element in the condition value array. The condition value MUST be an array.

### Requirement: "is_empty" operator checks for nil, empty string, or zero value

The "is_empty" operator SHALL evaluate to true when the field value is nil, an empty string, or a zero value. The condition value field is ignored for this operator.

#### Scenario: is_empty on nil value
- **WHEN** a gateway condition has operator="is_empty"
- **WHEN** the field value is nil
- **THEN** the condition evaluates to true

#### Scenario: is_empty on empty string
- **WHEN** a gateway condition has operator="is_empty"
- **WHEN** the field value is ""
- **THEN** the condition evaluates to true

### Requirement: "is_not_empty" operator is negation of is_empty

The "is_not_empty" operator SHALL evaluate to true when the field value is not nil, not an empty string, and not a zero value. The condition value field is ignored for this operator.

### Requirement: "between" operator checks numeric range

The "between" operator SHALL accept a condition value as a two-element array [min, max]. It SHALL evaluate to true when the field value is greater than or equal to min AND less than or equal to max. Both comparisons are numeric.

#### Scenario: between numeric range
- **WHEN** a gateway condition has operator="between" and value=[10, 50]
- **WHEN** the field value is 25
- **THEN** the condition evaluates to true

### Requirement: "matches" operator checks regex pattern

The "matches" operator SHALL accept a condition value as a regex pattern string. It SHALL evaluate to true when the field value matches the pattern. Invalid regex patterns SHALL cause the condition to return an error.

#### Scenario: matches regex pattern
- **WHEN** a gateway condition has operator="matches" and value="^INC-\\d{4}$"
- **WHEN** the field value is "INC-0042"
- **THEN** the condition evaluates to true

### Requirement: GatewayCondition supports compound conditions

GatewayCondition SHALL support compound conditions via a Logic field ("and" or "or") and a Conditions field ([]GatewayCondition). When Logic is set, the evaluator SHALL recursively evaluate all sub-conditions and combine results with the specified logical operator.

#### Scenario: Compound AND — all sub-conditions must match
- **WHEN** a gateway condition has logic="and" with two sub-conditions: priority="high" and category="incident"
- **WHEN** the ticket has priority="high" and category="incident"
- **THEN** the compound condition evaluates to true

#### Scenario: Compound OR — any sub-condition may match
- **WHEN** a gateway condition has logic="or" with two sub-conditions: priority="high" and priority="critical"
- **WHEN** the ticket has priority="critical"
- **THEN** the compound condition evaluates to true

#### Scenario: Nested compound conditions
- **WHEN** a gateway condition has logic="and" with sub-conditions: one is logic="or" (priority="high" OR priority="critical"), another is category="incident"
- **WHEN** the ticket has priority="critical" and category="incident"
- **THEN** the nested compound condition evaluates to true

### Requirement: Empty Logic field preserves backward compatibility

When the Logic field is empty or unset, the evaluator SHALL use the existing single-condition evaluation path (field + operator + value). This ensures backward compatibility with all previously defined gateway conditions.

#### Scenario: Backward compatible single condition
- **WHEN** a gateway condition has field="priority", operator="eq", value="high", and logic is empty
- **WHEN** the ticket has priority="high"
- **THEN** the condition evaluates to true using the single-condition path
