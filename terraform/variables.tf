variable "region" {
  description = "Region of AWS account"
  type        = string
  default     = "eu-north-1"
}

variable "db_username" {
  description = "Postgres DB username"
  type        = string
}

variable "db_password" {
  description = "Postgres DB password"
  type        = string
  sensitive   = true
}

variable "db_database" {
  description = "Postgres DB name"
  type        = string
}

variable "domain_name" {
  description = "Builder domain name"
  type        = string
}
