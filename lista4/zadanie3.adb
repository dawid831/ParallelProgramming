with Ada.Text_IO; use Ada.Text_IO;
with Ada.Numerics.Float_Random; use Ada.Numerics.Float_Random;
with Random_Seeds; use Random_Seeds;
with monitor_package; use monitor_package;
with Ada.Real_Time; use Ada.Real_Time;

procedure  zadanie3 is
-- Processes 

  Nr_Readers : constant := 10;
  Nr_Writers : constant := 5;
  Nr_Of_Processes : constant Integer := Nr_Readers + Nr_Writers;

  Min_Steps : constant Integer := 20 ;
  Max_Steps : constant Integer := 40 ;

  Min_Delay : constant Duration := 0.01;
  Max_Delay : constant Duration := 0.05;

  OK_to_Read, OK_to_Write: Condition;

  Readers: Integer := 0;
  Writing: Boolean := False;

  task type Reader is
    entry Init(I: Integer; Seed: Integer);
    entry Start;
  end Reader;

  task type Writer is
    entry Init(I: Integer; Seed: Integer);
    entry Start;
  end Writer;

  procedure Start_Read is
  begin
    Monitor.Enter;
    if Writing or Non_Empty(OK_to_Write) then
       FixedWait(OK_to_Read);
    end if;
    Readers := Readers + 1;
    OK_to_Read.Signal;
  end Start_Read;

  procedure Stop_Read is
  begin
    Monitor.Enter;
    Readers := Readers - 1;
    if Readers = 0 then
       OK_to_Write.Signal;
    else
       Monitor.Leave;
    end if;
  end Stop_Read;

  procedure Start_Write is
  begin
    Monitor.Enter;
    if Readers /= 0 or Writing then
       FixedWait(OK_to_Write);
    end if;
    Writing := True;
    Monitor.Leave;
  end Start_Write;

  procedure Stop_Write is
  begin
    Monitor.Enter;
    Writing := False;
    if Non_Empty(OK_to_Read) then
       OK_to_Read.Signal;
    else
       OK_to_Write.Signal;
    end if;
  end Stop_Write;

-- States of a Process 

  type Process_State is (
	Local_Section,
	Start,
	Reading_Room,
	Stop
	);

-- 2D Board display board

  Board_Width  : constant Integer := Nr_Of_Processes;
  Board_Height : constant Integer := Process_State'Pos( Process_State'Last ) + 1;

-- Timing

  Start_Time : Time := Clock;  -- global startnig time

-- Random seeds for the tasks' random number generators
 
  Seeds : Seed_Array_Type( 1..Nr_Of_Processes ) := Make_Seeds( Nr_Of_Processes );

-- Types, procedures and functions

  -- Postitions on the board
  type Position_Type is record	
    X: Integer range 0 .. Board_Width - 1; 
    Y: Integer range 0 .. Board_Height - 1; 
  end record;	   

  -- traces of Processes
  type Trace_Type is record 	      
    Time_Stamp:  Duration;	      
    Id : Integer;
    Position: Position_Type;      
    Symbol: Character;	      
  end record;	      

  type Trace_Array_type is  array(0 .. Max_Steps) of Trace_Type;

  type Traces_Sequence_Type is record
    Last: Integer := -1;
    Trace_Array: Trace_Array_type ;
  end record; 


  procedure Print_Trace( Trace : Trace_Type ) is
    Symbol : String := ( ' ', Trace.Symbol );
  begin
    Put_Line(
        Duration'Image( Trace.Time_Stamp ) & " " &
        Integer'Image( Trace.Id ) & " " &
        Integer'Image( Trace.Position.X ) & " " &
        Integer'Image( Trace.Position.Y ) & " " &
        ( ' ', Trace.Symbol ) -- print as string to avoid: '
      );
  end Print_Trace;

  procedure Print_Traces( Traces : Traces_Sequence_Type ) is
  begin
    for I in 0 .. Traces.Last loop
      Print_Trace( Traces.Trace_Array( I ) );
    end loop;
  end Print_Traces;

  -- task Printer collects and prints reports of traces and the line with the parameters

  task Printer is
    entry Report( Traces : Traces_Sequence_Type );
  end Printer;
  
  task body Printer is 
  begin
  
    -- Collect and print the traces
    
    for I in 1 .. Nr_Of_Processes loop -- range for TESTS !!!
        accept Report( Traces : Traces_Sequence_Type ) do
          -- Put_Line("I = " & I'Image );
          Print_Traces( Traces );
        end Report;
      end loop;

    -- Prit the line with the parameters needed for display script:

    Put(
      "-1 "&
      Integer'Image( Nr_Of_Processes ) &" "&
      Integer'Image( Board_Width ) &" "&
      Integer'Image( Board_Height ) &" "       
    );
    for I in Process_State'Range loop
      Put( I'Image &";" );
    end loop;

  end Printer;


  -- Processes
  task body Reader is
    ID: Integer;
    G : Generator;
    Nr_of_Steps: Integer;
    Traces: Traces_Sequence_Type; 
    Position: Position_Type;
    Time_Stamp : Duration;
    
    procedure Store_Trace is
    begin  
      Traces.Last := Traces.Last + 1;
      Traces.Trace_Array( Traces.Last ) := (
          Time_Stamp => Time_Stamp,
          Id => Id,
          Position => Position,
          Symbol => 'R'
        );
    end Store_Trace;
    
    procedure Change_State( State: Process_State ) is
    begin
      Time_Stamp := To_Duration ( Clock - Start_Time ); -- reads global clock
      Position.Y := Process_State'Pos( State );
      Store_Trace;
    end;
    
  begin
    accept Init(I: Integer; Seed: Integer) do
      Reset(G, Seed); 
      ID := I;
      -- Initial position 
      Position := (
          X => Id,
          Y => Process_State'Pos( LOCAL_SECTION )
        );
        -- Number of steps to be made by the Process  
      Nr_of_Steps := Min_Steps + Integer( Float(Max_Steps - Min_Steps) * Random(G));
      -- Time_Stamp of initialization
      Time_Stamp := To_Duration ( Clock - Start_Time ); -- reads global clock
      Store_Trace; -- store starting position
    end Init;
    
    -- wait for initialisations of the remaining tasks:
    accept Start do
      null;
    end Start;
    
    for Step in 0 .. Nr_of_Steps/4 - 1  loop
      -- LOCAL_SECTION - start
      delay Min_Delay+(Max_Delay-Min_Delay)*Duration(Random(G));
      -- LOCAL_SECTION - end
      
      Change_State( Start );
      Start_Read;
      
      Change_State( Reading_room );
      delay Min_Delay+(Max_Delay-Min_Delay)*Duration(Random(G));
      
      Change_State( Stop );
      Stop_Read;
      
      Change_State( Local_Section );
    end loop;
    Printer.Report( Traces );
  end Reader;

  task body Writer is
    ID: Integer;
    G : Generator;
    Nr_of_Steps: Integer;
    Traces: Traces_Sequence_Type; 
    Position: Position_Type;
    Time_Stamp : Duration;
    
    procedure Store_Trace is
    begin  
      Traces.Last := Traces.Last + 1;
      Traces.Trace_Array( Traces.Last ) := (
          Time_Stamp => Time_Stamp,
          Id => Id,
          Position => Position,
          Symbol => 'W'
        );
    end Store_Trace;
    
    procedure Change_State( State: Process_State ) is
    begin
      Time_Stamp := To_Duration ( Clock - Start_Time ); -- reads global clock
      Position.Y := Process_State'Pos( State );
      Store_Trace;
    end;
    
  begin
    accept Init(I: Integer; Seed: Integer) do
      Reset(G, Seed); 
      ID := I;
      -- Initial position 
      Position := (
          X => Id,
          Y => Process_State'Pos( LOCAL_SECTION )
        );
        -- Number of steps to be made by the Process  
      Nr_of_Steps := Min_Steps + Integer( Float(Max_Steps - Min_Steps) * Random(G));
      -- Time_Stamp of initialization
      Time_Stamp := To_Duration ( Clock - Start_Time ); -- reads global clock
      Store_Trace; -- store starting position
    end Init;
    
    -- wait for initialisations of the remaining tasks:
    accept Start do
      null;
    end Start;
    
    for Step in 0 .. Nr_of_Steps/4 - 1  loop
      -- LOCAL_SECTION - start
      delay Min_Delay+(Max_Delay-Min_Delay)*Duration(Random(G));
      -- LOCAL_SECTION - end

      Change_State( Start );
      Start_Write;

      Change_State( Reading_room );
      delay Min_Delay+(Max_Delay-Min_Delay)*Duration(Random(G));
        
      Change_State( Stop );
      Stop_Write;
      
      Change_State( Local_Section);
    end loop;
    Printer.Report( Traces );
  end Writer;

-- local for main task

  R: array (0..Nr_Readers-1) of Reader;
  W: array (0..Nr_Writers-1) of Writer;

begin 
  -- init tarvelers tasks
  for I in R'Range loop
    R(I).Init( I, Seeds(I+1) );   -- `Seeds(I+1)` is ugly :-(
  end loop;
  
  for I in W'Range loop
    W(I).Init( I+Nr_Readers, Seeds(Nr_Readers+I+1) );   -- `Seeds(I+1)` is ugly :-(
  end loop;

  -- start tarvelers tasks
  for I in R'Range loop
    R(I).Start;
  end loop;
  
  for I in W'Range loop
    W(I).Start;
  end loop;

end zadanie3;

