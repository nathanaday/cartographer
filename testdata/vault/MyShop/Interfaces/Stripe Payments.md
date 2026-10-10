---
id: ifc-str1pe
type: carto-interface
description: Card payments through Stripe, and Stripe's webhooks back to the shop.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: http
definition: Stripe API reference, version 2026-09-30
stability: stable
schemas: []
external: Stripe, the payment provider.
---

## Purpose

Takes the customer's payment at checkout.

## Wire

| | |
|---|---|
| Transport | HTTPS, JSON |
| Address | `https://api.stripe.com/v1`; webhooks to `https://api.myshop.com/v1/stripe/webhook` |
| Messages | PaymentIntent create and confirm. Webhook `payment_intent.succeeded` marks the order paid. |
| Auth | A secret key from Secret Manager. Each webhook carries a Stripe signature. |
| Versioning | The `Stripe-Version` header pins the API version. |

`Stripe API reference, version 2026-09-30` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[shop-api]]
> External: Stripe, the payment provider.

## Notes
