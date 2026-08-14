-- 088: the NEXT distance cohort must be online too.
--
-- 087 added course_offerings.delivery_mode and backfilled it from session_type, which fixed every
-- e-learning cohort that existed the day it ran. It did nothing for the ones created afterwards —
-- and cohorts are created every intake, by the admin screen and by the curriculum/timetable
-- imports alike. A new cohort typed "Distance Learning" would take the column default, IN_PERSON,
-- and the failure is silent in the worst way: the timetable looks right, the lecturer's start
-- button is refused with "this cohort is taught in person", and the students simply cannot check
-- in to a class that has no room to be near. Nobody would connect those symptoms to a column
-- nobody set.
--
-- WHY A TRIGGER RATHER THAN THE HANDLERS. Cohorts are inserted from at least four places (the
-- admin cohort form, the curriculum import, the timetable import, the cohort-apply tool). Putting
-- the rule in each of them is the drift this codebase has been bitten by before: three paths get
-- it and the fourth quietly does not, and which one you used decides whether your students have
-- attendance. The rule belongs next to the data it constrains.
--
-- THE ONE CAVEAT, stated rather than hidden: the trigger cannot distinguish "delivery_mode was not
-- supplied" from "delivery_mode was explicitly set to IN_PERSON", because the column is NOT NULL
-- with a default. So an insert that deliberately says IN_PERSON for a cohort named "Distance
-- Learning" is overridden. That combination is a contradiction on its face, the safe reading of it
-- is the one that lets those students attend, and it is correctable immediately: UPDATE is
-- untouched, so the admin screen's Delivery field always wins afterwards.

CREATE OR REPLACE FUNCTION offering_delivery_mode_default() RETURNS trigger AS $$
BEGIN
    IF NEW.delivery_mode = 'IN_PERSON'
       AND NEW.session_type ~* '(distance|e[[:space:]._-]*learning|online|virtual|remote)' THEN
        NEW.delivery_mode := 'ONLINE';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION offering_delivery_mode_default() IS
    'Sets delivery_mode = ONLINE for a newly created cohort whose session_type names it as a '
    'distance / e-learning run. INSERT only: an explicit UPDATE from the admin screen always wins.';

DROP TRIGGER IF EXISTS trg_offering_delivery_mode ON course_offerings;
CREATE TRIGGER trg_offering_delivery_mode
    BEFORE INSERT ON course_offerings
    FOR EACH ROW EXECUTE FUNCTION offering_delivery_mode_default();

-- Re-run the backfill for anything created between 087 and this migration.
UPDATE course_offerings
   SET delivery_mode = 'ONLINE'
 WHERE delivery_mode = 'IN_PERSON'
   AND session_type ~* '(distance|e[[:space:]._-]*learning|online|virtual|remote)';
