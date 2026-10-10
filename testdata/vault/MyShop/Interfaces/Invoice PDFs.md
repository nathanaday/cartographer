---
id: ifc-1nvpdf
type: carto-interface
description: One PDF invoice for each paid order, in a Cloud Storage bucket.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: report
definition: github.com/myshop/myshop-inventory/invoices/template.html
stability: stable
schemas: []
external: Accounting imports the PDFs into Xero each night.
---

## Purpose

The legal record of each sale, for accounting.

## Wire

| | |
|---|---|
| Transport | Cloud Storage |
| Address | `gs://myshop-invoices/{yyyy}/{mm}/{order_id}.pdf` |
| Messages | One PDF for each paid order, rendered from `invoices/template.html`. |
| Delivery | Written once and never changed. A correction is a new credit note. |
| Versioning | None. The template can change, and old PDFs stay as they are. |

`github.com/myshop/myshop-inventory/invoices/template.html` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[invoice-worker]]
> External: Accounting imports the PDFs into Xero each night.

## Notes
