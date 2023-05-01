resource "aws_acm_certificate" "mirage-ecs" {
  domain_name       = var.domain
  validation_method = "DNS"
  subject_alternative_names = [
    "*.${var.domain}"
  ]
  tags = {
    Name = var.project
  }
}

resource "aws_acm_certificate_validation" "mirage-ecs" {
  certificate_arn         = aws_acm_certificate.mirage-ecs.arn
  validation_record_fqdns = [for v in aws_route53_record.validation : v.fqdn]
}
