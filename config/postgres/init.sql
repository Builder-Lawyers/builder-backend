CREATE SCHEMA IF NOT EXISTS builder;

CREATE TABLE IF NOT EXISTS builder.sites (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    template_id SMALLINT NOT NULL,
    creator_id UUID NOT NULL,
    plan_id SMALLINT NOT NULL,
    subscription_id VARCHAR(60),
    status VARCHAR(30) NOT NULL,
    fields JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.users (
    id UUID PRIMARY KEY,
    stripe_id VARCHAR(60),
    status VARCHAR(60),
    first_name varchar(100),
    second_name varchar(100),
    email varchar(100) NOT NULL,
    created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.templates (
    id INTEGER GENERATED ALWAYS AS IDENTITY,
    name VARCHAR(100) NOT NULL,
    fields JSONB,
    styles VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS builder.outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event VARCHAR(200) NOT NULL,
    status SMALLINT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS builder.provisions (
    site_id BIGINT PRIMARY KEY,
    "type" VARCHAR(40) NOT NULL,
    status VARCHAR(40) NOT NULL,
    domain VARCHAR(80),
    cert_arn VARCHAR(120),
    cloudfront_id VARCHAR(60),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.mails (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "type" VARCHAR(60) NOT NULL,
    recipients VARCHAR(255) NOT NULL,
    subject VARCHAR(100),
    content TEXT,
    sent_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS builder.mail_templates (
    id SMALLINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "type" VARCHAR(60) NOT NULL,
    content TEXT
);

CREATE TABLE IF NOT EXISTS builder.payment_plans (
    id SMALLINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stripe_id VARCHAR(60) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL,
    price INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS builder.sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    refresh_token TEXT,
    issued_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS builder.confirmation_codes (
    code UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    email VARCHAR(100),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS builder.files (
    id UUID PRIMARY KEY
);

insert into builder.users (id, stripe_id, status, email, created_at) values ('021804b8-5071-7049-7034-8853ffd88039',  'cus_SzleNRbLmsHvcs','Confirmed', 'sanity@mailinator.com', CURRENT_TIMESTAMP);
insert into builder.templates(name, styles) VALUES ('template-v1', 'https://sanity-web.s3.eu-north-1.amazonaws.com/templates-builds/template-v1/_astro/style.CKGSaZmw.css');
insert into builder.templates(name) VALUES ('template-v2');
insert into builder.payment_plans(stripe_id, description, price) VALUES ('price_1S2g3TBUqUlKX6nYFU5mN5HW', 'Simple site with no separate domain', 800);
insert into builder.payment_plans(stripe_id, description, price) VALUES ('price_1S3d1JBUqUlKX6nYewiReS7I', 'Simple site with separate domain', 1300);
insert into builder.mail_templates(type, content) VALUES ('FreeTrialEnds', '<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Your trial is ending soon</title><meta name="viewport" content="width=device-width,initial-scale=1.0"/></head><body style="margin:0;padding:0;background-color:#f4f6f8;font-family:Arial,Helvetica,sans-serif;"><div style="display:none;max-height:0;overflow:hidden;">Your trial ends in {{.DaysUntilEnd}} day{{if ne .DaysUntilEnd 1}}s{{end}} — add a payment method to avoid interruption.</div><table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="center" style="padding:20px 12px;"><table width="600" cellpadding="0" cellspacing="0" role="presentation" style="background:#ffffff;border-radius:8px;overflow:hidden;box-shadow:0 2px 6px rgba(0,0,0,0.08);"><tr><td style="padding:24px 28px;background:linear-gradient(90deg,#2563eb,#06b6d4);color:#ffffff;"><h1 style="margin:0;font-size:20px;font-weight:700;">Trial ending in {{.DaysUntilEnd}} day{{if ne .DaysUntilEnd 1}}s{{end}}</h1></td></tr><tr><td style="padding:28px;"><p style="margin:0 0 16px 0;color:#0f172a;font-size:15px;line-height:1.5;">Hi {{if .CustomerFirstName}}{{.CustomerFirstName}}{{end}}{{if .CustomerSecondName}} {{.CustomerSecondName}}{{end}},</p><p style="margin:0 0 18px 0;color:#334155;font-size:15px;line-height:1.5;">Your free trial will end in <strong>{{.DaysUntilEnd}} day{{if ne .DaysUntilEnd 1}}s{{end}}</strong>. To continue using our services without interruption, please add or update your payment method by following the button below.</p><p style="margin:0 0 18px 0;color:#334155;font-size:15px;line-height:1.5;">If you do not provide payment details, the site created for you will be <strong>deactivated</strong>.</p><div style="text-align:center;margin:26px 0;"><a href="{{.PaymentURL}}" target="_blank" rel="noopener noreferrer" style="display:inline-block;padding:12px 22px;border-radius:8px;text-decoration:none;font-weight:600;background:linear-gradient(90deg,#2563eb,#06b6d4);color:#ffffff;">Add / Update Payment Method</a></div><p style="margin:0 0 16px 0;color:#94a3b8;font-size:13px;line-height:1.4;">If the button doesn''t work, copy and paste this link into your browser:<br/><a href="{{.PaymentURL}}" target="_blank" rel="noopener noreferrer" style="color:#2563eb;word-break:break-all;">{{.PaymentURL}}</a></p><hr style="border:none;border-top:1px solid #e6eef6;margin:22px 0;"/><p style="margin:0;color:#64748b;font-size:13px;line-height:1.4;">Need help? Contact our support at <a href="mailto:support@example.com" style="color:#2563eb;text-decoration:none;">support@example.com</a>.</p></td></tr><tr><td style="padding:18px 28px;background:#f8fafc;color:#94a3b8;font-size:12px;text-align:center;"><div>© {{.Year}} Lawyers-Builder. All rights reserved.</div></td></tr></table></td></tr></table></body></html>');
insert into builder.mail_templates(type, content) VALUES ('SiteCreated', '<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Your site is ready</title><meta name="viewport" content="width=device-width,initial-scale=1.0"/></head><body style="margin:0;padding:0;background-color:#f4f6f8;font-family:Arial,Helvetica,sans-serif;"><table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="center" style="padding:20px 12px;"><table width="600" cellpadding="0" cellspacing="0" role="presentation" style="background:#ffffff;border-radius:8px;overflow:hidden;box-shadow:0 2px 6px rgba(0,0,0,0.08);"><tr><td style="padding:24px 28px;background:linear-gradient(90deg,#2563eb,#06b6d4);color:#ffffff;"><h1 style="margin:0;font-size:20px;font-weight:700;">Your new site is live!</h1></td></tr><tr><td style="padding:28px;"><p style="margin:0 0 16px 0;color:#0f172a;font-size:15px;line-height:1.5;">Hi {{if .CustomerFirstName}}{{.CustomerFirstName}}{{end}}{{if .CustomerSecondName}} {{.CustomerSecondName}}{{end}},</p><p style="margin:0 0 18px 0;color:#334155;font-size:15px;line-height:1.5;">We’re excited to let you know that your site has been created and is now available online.</p><div style="text-align:center;margin:26px 0;"><a href="{{.SiteURL}}" target="_blank" rel="noopener noreferrer" style="display:inline-block;padding:12px 22px;border-radius:8px;text-decoration:none;font-weight:600;background:linear-gradient(90deg,#2563eb,#06b6d4);color:#ffffff;">Visit Your Site</a></div><p style="margin:0 0 18px 0;color:#334155;font-size:15px;line-height:1.5;">If you’d like to make changes, just log in to our website, navigate to your site, and click <strong>Modify</strong>.</p><p style="margin:0 0 16px 0;color:#94a3b8;font-size:13px;line-height:1.4;">If the button doesn’t work, copy and paste this link into your browser:<br/><a href="{{.SiteURL}}" target="_blank" rel="noopener noreferrer" style="color:#2563eb;word-break:break-all;">{{.SiteURL}}</a></p><hr style="border:none;border-top:1px solid #e6eef6;margin:22px 0;"/><p style="margin:0;color:#64748b;font-size:13px;line-height:1.4;">Need help? Contact our support at <a href="mailto:support@example.com" style="color:#2563eb;text-decoration:none;">support@example.com</a>.</p></td></tr><tr><td style="padding:18px 28px;background:#f8fafc;color:#94a3b8;font-size:12px;text-align:center;"><div>© {{.Year}} Lawyers-Builder. All rights reserved.</div></td></tr></table></td></tr></table></body></html>');
insert into builder.mail_templates(type, content) VALUES ('SiteDeactivated', '<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Site Deactivated</title><meta name="viewport" content="width=device-width,initial-scale=1.0"/></head><body style="margin:0;padding:0;background-color:#f4f6f8;font-family:Arial,Helvetica,sans-serif;"><table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="center" style="padding:20px 12px;"><table width="600" cellpadding="0" cellspacing="0" role="presentation" style="background:#ffffff;border-radius:8px;overflow:hidden;box-shadow:0 2px 6px rgba(0,0,0,0.08);"><tr><td style="padding:24px 28px;background:#dc2626;color:#ffffff;"><h1 style="margin:0;font-size:20px;font-weight:700;">Site Deactivated</h1></td></tr><tr><td style="padding:28px;"><p style="margin:0 0 16px 0;color:#0f172a;font-size:15px;line-height:1.5;">Hi {{if .CustomerFirstName}}{{.CustomerFirstName}}{{end}}{{if .CustomerSecondName}} {{.CustomerSecondName}}{{end}},</p><p style="margin:0 0 16px 0;color:#334155;font-size:15px;line-height:1.5;">Your site <a href="{{.SiteURL}}" style="color:#2563eb;text-decoration:none;">{{.SiteURL}}</a> has been <strong>deactivated</strong>.</p><p style="margin:0 0 18px 0;color:#334155;font-size:15px;line-height:1.5;"><strong>Reason:</strong> {{.Reason}}</p><p style="margin:0 0 18px 0;color:#334155;font-size:15px;line-height:1.5;">If you believe this was a mistake or wish to reactivate your site, please log in to your account and review the status, or contact our support team.</p><hr style="border:none;border-top:1px solid #e6eef6;margin:22px 0;"/><p style="margin:0;color:#64748b;font-size:13px;line-height:1.4;">Need help? Contact our support at <a href="mailto:support@example.com" style="color:#2563eb;text-decoration:none;">support@example.com</a>.</p></td></tr><tr><td style="padding:18px 28px;background:#f8fafc;color:#94a3b8;font-size:12px;text-align:center;"><div>© {{.Year}} Lawyers-Builder. All rights reserved.</div></td></tr></table></td></tr></table></body></html>');
insert into builder.mail_templates(type, content) VALUES ('RegistrationConfirm', '<!doctype html><html><head><meta charset="UTF-8"/><title>Confirm your registration</title></head><body style="font-family:Arial,sans-serif;background:#f5f5f5;padding:20px;"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center"><table role="presentation" width="600" cellspacing="0" cellpadding="20" style="background:#ffffff;border-radius:8px;"><tr><td><h2 style="margin-bottom:16px;">Confirm your registration</h2><p style="margin-bottom:24px;">To complete your registration, please click the button below.</p><p style="text-align:center;margin:30px 0;"><a href="{{.RedirectURL}}" style="background:#007bff;color:#ffffff;text-decoration:none;padding:14px 24px;border-radius:5px;display:inline-block;">Confirm registration</a></p><p style="font-size:14px;color:#666;">If the button is not clickable, copy and open this link in your browser:<br><span style="word-break:break-all;">{{.RedirectURL}}</span></p><p style="font-size:12px;color:#999;margin-top:40px;">© {{.Year}} Lawyers-Builder. All rights reserved.</p></td></tr></table></td></tr></table></body></html>');

