// Fail-closed registration guard: an exception/timeout denies registration.
// The admin panel has an audited break-glass switch for emergencies.
skin.guards.on("user.registration.check", function (e) {
  var domain = String(skin.config.get("blockedDomain") || "").toLowerCase();
  var email = String((e.data && e.data.email) || "").toLowerCase();
  if (domain && email.endsWith("@" + domain)) {
    return {
      allow: false,
      code: "signup.blocked_domain",
      message: String(skin.config.get("allowMessage") || "该邮箱域名不允许注册")
    };
  }
  return { allow: true };
}, { onError: "deny", timeoutMS: 50 });
