-- Migration 010: Add AUTHENTICATED entry method for JWT-based student check-in.
-- The online check-in flow uses email+password login (identity) + room code
-- (proximity) instead of a QR scan, so it needs its own entry_method value.

ALTER TYPE entry_method_enum ADD VALUE IF NOT EXISTS 'AUTHENTICATED';
