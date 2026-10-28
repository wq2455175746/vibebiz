package model

import "time"

type User struct {
	Base
	Email           string     `gorm:"column:email" json:"email"`
	Source          string     `gorm:"column:source" json:"source"`
	Username        string     `gorm:"column:username" json:"username"`
	PasswordHash    *string    `gorm:"column:password_hash" json:"-"`
	Nickname        string     `gorm:"column:nickname" json:"nickname"`
	PhoneNumber     string     `gorm:"column:phone_number" json:"phoneNumber"`
	AvatarURL       string     `gorm:"column:avatar_url" json:"avatarUrl"`
	AvatarS3Path    string     `gorm:"column:avatar_s3_path" json:"avatarS3Path"`
	Status          int8       `gorm:"column:status" json:"status"`
	EmailVerifiedAt *time.Time `gorm:"column:email_verified_at" json:"emailVerifiedAt,omitempty"`
	LastLoginTime   *time.Time `gorm:"column:last_login_time" json:"lastLoginTime,omitempty"`
	LastLoginIP     string     `gorm:"column:last_login_ip" json:"lastLoginIp"`
}

func (User) TableName() string { return "user" }

type OpsUser struct {
	Base
	Username      string     `gorm:"column:username" json:"username"`
	Email         string     `gorm:"column:email" json:"email"`
	PasswordHash  *string    `gorm:"column:password_hash" json:"-"`
	Nickname      string     `gorm:"column:nickname" json:"nickname"`
	PhoneNumber   string     `gorm:"column:phone_number" json:"phoneNumber"`
	AvatarURL     string     `gorm:"column:avatar_url" json:"avatarUrl"`
	RoleCode      string     `gorm:"column:role_code" json:"roleCode"`
	Source        string     `gorm:"column:source" json:"source"`
	Status        int8       `gorm:"column:status" json:"status"`
	LastLoginTime *time.Time `gorm:"column:last_login_time" json:"lastLoginTime,omitempty"`
	LastLoginIP   string     `gorm:"column:last_login_ip" json:"lastLoginIp"`
	CreatedBy     string     `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy     string     `gorm:"column:updated_by" json:"updatedBy"`
}

func (OpsUser) TableName() string { return "ops_user" }

type OpsAuditLog struct {
	Base
	OpsUserID     *int64 `gorm:"column:ops_user_id" json:"opsUserId,omitempty"`
	ActorName     string `gorm:"column:actor_name" json:"actorName"`
	Action        string `gorm:"column:action" json:"action"`
	ResourceType  string `gorm:"column:resource_type" json:"resourceType"`
	ResourceID    string `gorm:"column:resource_id" json:"resourceId"`
	RequestMethod string `gorm:"column:request_method" json:"requestMethod"`
	RequestPath   string `gorm:"column:request_path" json:"requestPath"`
	RequestID     string `gorm:"column:request_id" json:"requestId"`
	TraceID       string `gorm:"column:trace_id" json:"traceId"`
	IP            string `gorm:"column:ip" json:"ip"`
	UserAgent     string `gorm:"column:user_agent" json:"userAgent"`
	BeforeData    JSON   `gorm:"column:before_data;type:json" json:"beforeData,omitempty"`
	AfterData     JSON   `gorm:"column:after_data;type:json" json:"afterData,omitempty"`
	ResultStatus  string `gorm:"column:result_status" json:"resultStatus"`
	ErrorMessage  string `gorm:"column:error_message" json:"errorMessage"`
}

func (OpsAuditLog) TableName() string { return "ops_audit_log" }
