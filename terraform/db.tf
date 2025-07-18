data "aws_rds_orderable_db_instance" "custom-postgres" {
  engine = "postgres"
  engine_version = "17.2"
  storage_type   = "gp2"
  instance_class = "db.t4g.micro"
  preferred_instance_classes = ["db.t4g.micro"]
}

resource "aws_db_instance" "postgres" {
  id = "test postgres rds"
  allocated_storage    = 20
  engine               = data.aws_rds_orderable_db_instance.custom-postgres.engine
  engine_version       = data.aws_rds_orderable_db_instance.custom-postgres.engine_version
  instance_class = data.aws_rds_orderable_db_instance.custom-postgres.instance_class
  multi_az             = false
  publicly_accessible = true
  db_name              = var.db_database
  password             = var.db_password
  username             = var.db_username
  storage_encrypted    = false

  skip_final_snapshot = true
  timeouts {
    create = "3h"
    delete = "3h"
    update = "3h"
  }
}