# Preset workflows are explicit and plan-backed

Preset workflows will support local preset files through the same Change Plan, apply, and save model used for settings.
Network preset fetching is opt-in and must report source metadata such as URL, version, checksum when available, and retrieval time.
Preset application should never be an implicit network side effect or an unreviewed write.
