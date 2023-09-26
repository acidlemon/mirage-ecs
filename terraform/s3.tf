resource "aws_s3_bucket" "mirage-ecs" {
  bucket = format("mirage-%s", replace(var.domain, ".", "-"))
}

resource "aws_s3_object" "config" {
  bucket = aws_s3_bucket.mirage-ecs.id
  key    = "config.yaml"
  source = "config.yaml"
}

resource "aws_s3_object" "html" {
  for_each = toset(["launcher.html", "layout.html", "list.html"])
  bucket   = aws_s3_bucket.mirage-ecs.id
  key      = "html/${each.value}"
  source   = format("../html/%s", each.value)
}
