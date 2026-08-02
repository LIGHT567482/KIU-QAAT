-- Migration 039: Remove BLE beacon proximity entirely.
--
-- The attendance proximity model is now LAN-only: a student or lecturer can only
-- log attendance while connected to the coordinator's hotspot (same LAN), proven
-- by onSameLAN() against the session's coordinator_ip, plus the rotating room code
-- read live off the coordinator's screen. Per-student BLE RSSI was never reliably
-- readable from a browser (no Web Bluetooth on iOS Safari) and is now removed.
--
-- This drops: the beacon registry table, its enum type, the per-record RSSI
-- column, and the configurable RSSI threshold. CASCADE clears the RLS policies
-- that were attached to ble_beacons.
--
-- KEEP THIS FILE. Migration 001 no longer creates any of these objects, so on a
-- fresh database every statement below is a no-op (all are IF EXISTS). It still
-- has to run against databases provisioned before 001 was cleaned up — that is
-- the only path by which these objects can exist at all.

DROP TABLE IF EXISTS ble_beacons CASCADE;
DROP TYPE  IF EXISTS beacon_format_enum;

ALTER TABLE attendance_logs DROP COLUMN IF EXISTS beacon_rssi;
ALTER TABLE tenants         DROP COLUMN IF EXISTS rssi_threshold_dbm;
