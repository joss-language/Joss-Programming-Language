package core

import "fmt"

// EnsureMFATables creates MFA related tables if they don't exist
func (r *Runtime) EnsureMFATables() {
	if r.GetDB() == nil {
		return
	}

	// 1. Create MFA Methods Table
	if err := r.ensureInternalSchemaTable("user_mfa_methods", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "user_id", definition: "bigInteger"},
		{name: "method_type", definition: "string(50)"},
		{name: "secret", definition: "text|nullable"},
		{name: "is_active", definition: "boolean|default(0)"},
		{name: "created_at", definition: "timestamp|nullable"},
		{name: "updated_at", definition: "timestamp|nullable"},
	}); err != nil {
		fmt.Printf("[MFA] Error creando user_mfa_methods: %v\n", err)
		return
	}

	// 2. Create Recovery Codes Table
	if err := r.ensureInternalSchemaTable("user_recovery_codes", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "user_id", definition: "bigInteger"},
		{name: "code_hash", definition: "string(255)"},
		{name: "used", definition: "boolean|default(0)"},
		{name: "used_at", definition: "timestamp|nullable"},
		{name: "created_at", definition: "timestamp|nullable"},
		{name: "updated_at", definition: "timestamp|nullable"},
	}); err != nil {
		fmt.Printf("[MFA] Error creando user_recovery_codes: %v\n", err)
		return
	}

	// 3. Create Security Logs Table
	if err := r.ensureInternalSchemaTable("security_logs", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "user_id", definition: "bigInteger|nullable"},
		{name: "event_type", definition: "string(100)"},
		{name: "ip_address", definition: "string(45)|nullable"},
		{name: "user_agent", definition: "text|nullable"},
		{name: "created_at", definition: "timestamp|nullable"},
	}); err != nil {
		fmt.Printf("[MFA] Error creando security_logs: %v\n", err)
	}

	// 4. Create MFA Challenges Table (for Email OTP and session challenges)
	if err := r.ensureInternalSchemaTable("user_mfa_challenges", []schemaColumn{
		{name: "id", definition: "increments"},
		{name: "user_id", definition: "bigInteger"},
		{name: "method_type", definition: "string(50)"},
		{name: "code_hash", definition: "string(255)"},
		{name: "expires_at", definition: "timestamp|nullable"},
		{name: "used", definition: "boolean|default(0)"},
		{name: "created_at", definition: "timestamp|nullable"},
	}); err != nil {
		fmt.Printf("[MFA] Error creando user_mfa_challenges: %v\n", err)
	}
}
