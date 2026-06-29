-- Migration 043: optional tenant background IMAGE.
--
-- A tenant can now choose to use either a background colour (existing
-- background_color) OR a background image. When background_image is set it takes
-- precedence; otherwise the colour is used. Stored like logo_url (https URL or a
-- base64 image data URL).

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS background_image TEXT;
