resource "aws_route53_zone" "mirage-ecs" {
  name = var.domain
}

resource "aws_route53_record" "mirage-ecs" {
  zone_id = aws_route53_zone.mirage-ecs.zone_id
  name    = "mirage.${var.domain}"
  type    = "A"
  alias {
    name                   = aws_lb.mirage-ecs.dns_name
    zone_id                = aws_lb.mirage-ecs.zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "mirage-tasks" {
  zone_id = aws_route53_zone.mirage-ecs.zone_id
  name    = "*.${var.domain}"
  type    = "A"
  alias {
    name                   = aws_lb.mirage-ecs.dns_name
    zone_id                = aws_lb.mirage-ecs.zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "validation" {
  zone_id = aws_route53_zone.mirage-ecs.zone_id
  for_each = {
    for dvo in aws_acm_certificate.mirage-ecs.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }
  name            = each.value.name
  records         = [each.value.record]
  type            = each.value.type
  allow_overwrite = true
  ttl             = 60
}
