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
