---
id: ifc-r3cmd1
type: carto-interface
description: Model versions in a Cloud Storage bucket, each with a model card, and a marker for the live one.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: file
definition: github.com/myshop/myshop-recs/trainer/export.py
stability: experimental
schemas: []
external: ""
---

## Purpose

Moves a trained model from the trainer to the server.

## Wire

| | |
|---|---|
| Transport | Cloud Storage |
| Address | `gs://myshop-models/recs/{version}/`, and `gs://myshop-models/recs/live.json` |
| Messages | Each version holds `model.pt` and `model_card.json`. `live.json` names the live version. |
| Delivery | The trainer writes the whole version first and changes `live.json` last, so a reader never sees half a version. |
| Versioning | Experimental: the model card format may still change. |

`github.com/myshop/myshop-recs/trainer/export.py` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[recs-server]] · [[recs-trainer]]

## Notes
