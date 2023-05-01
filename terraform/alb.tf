resource "aws_lb" "mirage-ecs" {
  name               = var.project
  internal           = false
  load_balancer_type = "application"
  security_groups = [
    aws_security_group.alb.id,
    aws_security_group.default.id,
  ]
  subnets = [
    aws_subnet.public-a.id,
    aws_subnet.public-c.id,
    aws_subnet.public-d.id,
  ]
  tags = {
    Name = var.project
  }
}

resource "aws_lb_target_group" "mirage-ecs-http" {
  name                 = "${var.project}-http"
  port                 = 80
  target_type          = "ip"
  vpc_id               = aws_vpc.main.id
  protocol             = "HTTP"
  deregistration_delay = 10

  health_check {
    path                = "/"
    port                = "traffic-port"
    protocol            = "HTTP"
    healthy_threshold   = 2
    unhealthy_threshold = 10
    timeout             = 5
    interval            = 6
  }
  tags = {
    Name = "${var.project}-http"
  }
}

resource "aws_lb_listener" "mirage-ecs-http" {
  load_balancer_arn = aws_lb.mirage-ecs.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
  tags = {
    Name = "${var.project}-https"
  }
}

resource "aws_lb_listener" "mirage-ecs-https" {
  load_balancer_arn = aws_lb.mirage-ecs.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate.mirage-ecs.arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.mirage-ecs-http.arn
  }

  tags = {
    Name = "${var.project}-https"
  }
}
