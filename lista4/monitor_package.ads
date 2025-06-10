-- Copyright (C) 2006 M. Ben-Ari. See copyright.txt
-- Modified version
package Monitor_Package is

  task Monitor is
    entry Enter;
    entry Leave;
  end Monitor;

  task type Condition is
    entry Signal;
    entry Pre_Wait;
    entry Wait;
    entry Waiting(B: out Boolean);
  end Condition;

  function Non_Empty(C: Condition) return Boolean;
  procedure FixedWait(C: in out Condition);

end Monitor_Package;
