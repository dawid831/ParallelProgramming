with Ada.Text_IO; use Ada.Text_IO;
with Ada.Numerics.Float_Random; use Ada.Numerics.Float_Random;
with Random_Seeds; use Random_Seeds;
with Ada.Real_Time; use Ada.Real_Time;
with Ada.Synchronous_Task_Control; use Ada.Synchronous_Task_Control;

procedure Zadanie3 is

  -- Travelers moving on the board
  Nr_Of_Travelers : constant Integer := 15;
  Nr_Of_Wild_Tenants : constant Integer := 10;  -- Number of wild tenants
  Min_Steps : constant Integer := 10;
  Max_Steps : constant Integer := 100;
  Min_Delay : constant Duration := 0.01;
  Max_Delay : constant Duration := 0.05;
  Wild_Tenant_Lifetime : constant Duration := 0.5;  -- Lifetime of wild tenants
  Traps_Count : constant Integer := 10;

  -- 2D Board with torus topology
  Board_Width  : constant Integer := 15;
  Board_Height : constant Integer := 15;

  -- Timing
  Start_Time : Time := Clock;  -- global starting time

  -- Random seeds for the tasks' random number generators
  Seeds : Seed_Array_Type(1..Nr_Of_Travelers) := Make_Seeds(Nr_Of_Travelers);
  Wild_Tenant_Seeds : Seed_Array_Type(1..Nr_Of_Wild_Tenants) := Make_Seeds(Nr_Of_Wild_Tenants);

  -- Types, procedures and functions
  type Position_Type is record
    X: Integer range -1 .. Board_Width - 1;
    Y: Integer range -1 .. Board_Height - 1;
  end record;

  -- travelers
  type Player_Type is record
    Id: Integer;
    Symbol: Character;
    Position: Position_Type;
    Wild: Boolean;
  end record;
  
  Players : array (0 .. Nr_Of_Travelers + Nr_Of_Wild_Tenants - 1) of Player_Type;
  
   protected type Atomic_Counter is
      procedure Increment;
      procedure Decrement;
      function Get_Count return Natural;
   private
      Count : Natural := 0;
   end Atomic_Counter;

   protected body Atomic_Counter is
      procedure Increment is
      begin
         Count := Count + 1;
      end Increment;

      procedure Decrement is
      begin
         Count := Count - 1;
      end Decrement;

      function Get_Count return Natural is
      begin
         return Count;
      end Get_Count;
   end Atomic_Counter;
   
     -- Atomic counters for finalising some procedures
  Active_Travelers     : Atomic_Counter;
  Active_Wild_Tenants  : Atomic_Counter;

-- traces of travelers and wild tenants
  type Trace_Type is record
    Time_Stamp: Duration;
    Id : Integer;
    Position: Position_Type;
    Symbol: Character;
  end record;

  type Trace_Array_type is array(0 .. Max_Steps + 100) of Trace_Type;  -- Increased for wild tenants

  type Traces_Sequence_Type is record
    Last: Integer := -1;
    Trace_Array: Trace_Array_type;
  end record;

  procedure Print_Trace(Trace : Trace_Type) is
  begin
    Put_Line(
        Duration'Image(Trace.Time_Stamp) & " " &
        Integer'Image(Trace.Id) & " " &
        Integer'Image(Trace.Position.X) & " " &
        Integer'Image(Trace.Position.Y) & " " &
        (1 => Trace.Symbol)  -- print as string to avoid: '
      );
  end Print_Trace;

  procedure Print_Traces(Traces : Traces_Sequence_Type) is
  begin
    for I in 0 .. Traces.Last loop
      Print_Trace(Traces.Trace_Array(I));
    end loop;
  end Print_Traces;

  -- task Printer collects and prints reports of traces
  task Printer is
    entry Report(Traces : Traces_Sequence_Type);
  end Printer;

  task body Printer is
  begin
    for I in 1 .. (Nr_Of_Travelers + Nr_Of_Wild_Tenants + Board_Width * Board_Height) loop
      accept Report(Traces : Traces_Sequence_Type) do
        Print_Traces(Traces);
      end Report;
    end loop;
  end Printer;

-- type for a single cell to indicate its availability
protected type Cell is
    entry Lock;
    procedure Unlock;
    procedure Export_Traces(To : out Traces_Sequence_Type);
    function Is_Locked return Boolean;

    -- Wild tenant operations
    procedure Occupy(Mover : Integer);
    function Is_Occupied return Boolean;
    function Get_Occupant return Integer;
    procedure Clear;
    procedure Move_Wild_Tenant(cell_X: Integer; cell_Y: Integer; Success: out Boolean);

    -- Traps operaions
    procedure Set_Trap(ID : Integer; X : Integer; Y : Integer);
    procedure Reset_Trap;
    function Is_Trapped return Boolean;

  private
    Locked : Boolean := False;
    Occupied : Boolean := False;
    Trapped : Boolean := False;
    Pos : Position_Type;
    Traps_ID : Integer;
    Occupant: Integer;
    Traces: Traces_Sequence_Type;
end Cell;

  type Board_Type is array (0 .. Board_Width-1, 0 .. Board_Height-1) of Cell;
  Board : Board_Type;

   protected body Cell is
      -- Used for moving wild tenants and traps
      procedure Store_Trace is
      begin
        Traces.Last := Traces.Last + 1;
        if Occupied then
         Traces.Trace_Array(Traces.Last) := (
            Time_Stamp => To_Duration(Clock - Start_Time),
            Id => Players(Occupant).Id,
            Position => Players(Occupant).Position,
            Symbol => Players(Occupant).Symbol
         );
        else
            Traces.Trace_Array(Traces.Last) := (
            Time_Stamp => To_Duration(Clock - Start_Time),
            Id => Traps_ID,
            Position => Pos,
            Symbol => '#'
         );
        end if;
      end Store_Trace;
      -- Entry for lock needed to interact with the cell
      entry Lock when not Locked is
      begin
    	Locked := True;
      end Lock;
    
    
      -- Every command must be executed when cell is locked!
      procedure Unlock is
      begin
        Locked := False;
      end Unlock;
      
      procedure Export_Traces(To : out Traces_Sequence_Type) is
      begin
  	To := Traces;
      end Export_Traces;
    
      function Is_Locked return Boolean is
      begin
         return Locked;
      end Is_Locked;

      procedure Occupy(Mover: Integer) is
      begin
        Occupant := Mover;
        Occupied := True;
      end Occupy;
      
      function Is_Occupied return Boolean is
      begin
        return Occupied;
      end;
      
      function Get_Occupant return Integer is
      begin
        return Occupant;
      end Get_Occupant;
      
      procedure Clear is
      begin
        Occupied := False;
      end Clear;
    
      procedure Move_Wild_Tenant(cell_X: Integer; cell_Y: Integer; Success: out Boolean) is
      	begin 	
      	  if Occupied and Players(Occupant).Wild then
            declare
               Directions : constant array(1..4) of Position_Type := (
                  (X => (cell_X + 1) mod Board_Width, Y => cell_Y),
                  (X => (cell_X + Board_Width - 1) mod Board_Width, Y => cell_Y),
                  (X => cell_X, Y => (cell_Y + 1) mod Board_Height),
                  (X => cell_X, Y => (cell_Y + Board_Height - 1) mod Board_Height)
               );
            begin
               for Dir of Directions loop
                if not Board(Dir.X, Dir.Y).Is_Occupied then
                    select
                        Board(Dir.X, Dir.Y).Lock;
                        if not Board(Dir.X, Dir.Y).Is_Occupied then
                           Board(Dir.X, Dir.Y).Occupy(Occupant);
                           Players(Occupant).Position := Dir;
                           if Board(Dir.X, Dir.Y).Is_Trapped then
                              Players(Occupant).Symbol := '*';
                              Board(Dir.X, Dir.Y).Reset_Trap;
                              delay Max_Delay;
                              Players(Occupant).Position.X := -1;
                              Players(Occupant).Position.Y := -1;
                              Board(Dir.X, Dir.Y).Clear;
                              Board(Dir.X, Dir.Y).Reset_Trap;
                           end if;
                           Store_Trace;
                           Occupied := False;
                           Board(Dir.X, Dir.Y).Unlock;
                           -- Moved successfully
                           Success := True;
                           return;
                        else
                          -- Failsafe for when someone moves between initial check and lock
                          Board(Dir.X, Dir.Y).Unlock;
                        end if;
                    or
                        delay Max_Delay/10.0;
                    end select;
                end if;
              end loop;
              -- Loop ended - move was not successful
              Success := False;
              return;
            end;
          end if;
          -- Else - nothing to move, free to occupy
          -- Put_Line("ELSE");
          Success := True;
          return;
      	end Move_Wild_Tenant;  
  

      procedure Set_Trap(ID : Integer; X : Integer; Y : Integer) is
      begin
         Trapped := True;
         Pos := (X => X, Y => Y);
         Traps_ID := ID;
         Store_Trace;
      end Set_Trap;

      procedure Reset_Trap is
      begin
         Store_Trace;
      end Reset_Trap;

      function Is_Trapped return Boolean is
      begin
         return Trapped;
      end Is_Trapped;
   end Cell;
   

task type Traveler_Task_Type is
    entry Init(Id: Integer; Seed: Integer; Symbol: Character);
    entry Start;
  end Traveler_Task_Type;

  task body Traveler_Task_Type is
    G : Generator;
    Traveler : Integer;
    Time_Stamp : Duration;
    Nr_of_Steps: Integer;
    Traces: Traces_Sequence_Type;

    procedure Store_Trace is
    begin
      Time_Stamp := To_Duration(Clock - Start_Time);
      Traces.Last := Traces.Last + 1;
      Traces.Trace_Array(Traces.Last) := (
          Time_Stamp => Time_Stamp,
          Id => Players(Traveler).Id,
          Position => Players(Traveler).Position,
          Symbol => Players(Traveler).Symbol
        );
    end Store_Trace;

    procedure Make_Step is
      N : Integer;
      New_X, New_Y : Integer;
    begin
      N := Integer(Float'Floor(4.0 * Random(G)));
      case N is
        when 0 => 
          New_X := Players(Traveler).Position.X;
          New_Y := (Players(Traveler).Position.Y + Board_Height - 1) mod Board_Height;
        when 1 =>
          New_X := Players(Traveler).Position.X;
          New_Y := (Players(Traveler).Position.Y + 1) mod Board_Height;
        when 2 =>
          New_X := (Players(Traveler).Position.X + Board_Width - 1) mod Board_Width;
          New_Y := Players(Traveler).Position.Y;
        when 3 =>
          New_X := (Players(Traveler).Position.X + 1) mod Board_Width;
          New_Y := Players(Traveler).Position.Y;
        when others => 
          New_X := Players(Traveler).Position.X;
          New_Y := Players(Traveler).Position.Y;
      end case;
      
      declare
      	Original_X : Integer := Players(Traveler).Position.X;
        Original_Y : Integer := Players(Traveler).Position.Y;
        Success : Boolean := False;
      begin
        -- Try to lock the destination cell
        select
          Board(New_X, New_Y).Lock;
          Success := True;
        or
          delay Max_Delay * 2.0;
          Success := False;
        end select;

        if Success then
          -- Check if target has a wild tenant and try to move him if so
          
          Board(New_X, New_Y).Move_Wild_Tenant(New_X, New_Y, Success);
          if Success then
            -- Cell is empty and can be occupied
            Board(New_X, New_Y).occupy(Traveler);
            Players(Traveler).Position.X := New_X;
            Players(Traveler).Position.Y := New_Y;
            if Board(New_X, New_Y).Is_Trapped then
               Players(Traveler).Symbol := Character'Val(Character'Pos(Players(Traveler).Symbol) + 32);
               Store_Trace;
               delay Max_Delay;
               Players(Traveler).Position.X := -1;
               Players(Traveler).Position.Y := -1;
               Board(New_X, New_Y).Clear;
               Board(New_X, New_Y).Reset_Trap;
               Board(New_X, New_Y).Unlock;
            end if;
            Store_Trace;
            Board(Original_X, Original_Y).Unlock;
          else
            -- Failed to move a wild tenant, turn to small letter
            if Players(Traveler).Symbol in 'A' .. 'Z' then
              Players(Traveler).Symbol := Character'Val(Character'Pos(Players(Traveler).Symbol) + 32);
            end if;
            Store_Trace;
            Board(New_X, New_Y).Unlock;
          end if;
        else
          -- Convert to small letter if first time fail to lock
          if Players(Traveler).Symbol in 'A' .. 'Z' then
            Players(Traveler).Symbol := Character'Val(Character'Pos(Players(Traveler).Symbol) + 32);
          end if;
          Store_Trace;
        end if;
       end;
    end Make_Step;

  begin
    accept Init(Id: Integer; Seed: Integer; Symbol: Character) do
      Reset(G, Seed);
      Traveler := Id;
      Players(Traveler).Id := Id;
      Players(Traveler).Symbol := Symbol;
      Players(Traveler).Wild := False;
      -- Random initial position:
      loop
        Players(Traveler).Position := (
            X => Integer(Float'Floor(Float(Board_Width) * Random(G))),
            Y => Integer(Float'Floor(Float(Board_Height) * Random(G)))
          );
        select
          Board(Players(Traveler).Position.X, Players(Traveler).Position.Y).Lock;
          exit;
        or delay 0.001;
          null;
        end select;
      end loop;
      Store_Trace;
      Nr_of_Steps := Min_Steps + Integer(Float(Max_Steps - Min_Steps) * Random(G));
      Active_Travelers.Increment;
    end Init;

    accept Start do
      null;
    end Start;

    for Step in 0 .. Nr_of_Steps loop
      delay Min_Delay + (Max_Delay - Min_Delay) * Duration(Random(G));
      Make_Step;
      exit when Players(Traveler).Symbol not in 'A' .. 'Z';
    end loop;
    if Players(Traveler).Symbol in 'A' .. 'Z' then
      Players(Traveler).Symbol := Character'Val(Character'Pos(Players(Traveler).Symbol) + 32);
      Store_Trace;
    end if;
    Active_Travelers.Decrement;
    Printer.Report(Traces);
  end Traveler_Task_Type;

  task type Wild_Tenant_Task_Type is
    entry Init(Id: Integer; Seed: Integer);
    entry Start;
  end Wild_Tenant_Task_Type;

  
  task body Wild_Tenant_Task_Type is
    G : Generator;
    Wild_Tenant : Integer;
    Time_Stamp : Duration;
    Traces : Traces_Sequence_Type;
    Alive : Boolean;
    Birth_Time: Time;

    procedure Store_Trace is
    begin
      Time_Stamp := To_Duration(Clock - Start_Time);
      Traces.Last := Traces.Last + 1;
      Traces.Trace_Array(Traces.Last) := (
          Time_Stamp => Time_Stamp,
          Id => Players(Wild_Tenant).Id,
          Position => Players(Wild_Tenant).Position,
          Symbol => Players(Wild_Tenant).Symbol
        );
    end Store_Trace;

    procedure Safe_Appear is
      X, Y : Integer;
      Success : Boolean := False;
    begin
      for Attempt in 1..10 loop  -- Try 10 random positions
        X := Integer(Float'Floor(Float(Board_Width) * Random(G)));
        Y := Integer(Float'Floor(Float(Board_Height) * Random(G)));
        
        select
          Board(X, Y).Lock;
          if not Board(X, Y).Is_Trapped then
            Players(Wild_Tenant).Position.X := X;
            Players(Wild_Tenant).Position.Y := Y;
            Players(Wild_Tenant).Symbol := Character'Val(Character'Pos('0') + Players(Wild_Tenant).Id mod 10);
            Board(X, Y).Occupy(Wild_Tenant);
            Alive := True;
            Birth_Time := Clock;
            Time_Stamp := To_Duration(Clock - Start_Time);
            Store_Trace;  -- Record appearance
            Success := True;
            Board(X, Y).Unlock;
            exit;
         else
            Board(X, Y).Unlock;
         end if;
        or
          delay Max_Delay;
        end select;
      end loop;
      
      if not Success then
        -- Couldn't find a free spot, try again later
        delay Wild_Tenant_Lifetime / 10.0;
      end if;
    end Safe_Appear;
    
    procedure Safe_Disappear is
    X, Y : Integer;
      begin
      	 loop
            X := Players(Wild_Tenant).Position.X;
            Y := Players(Wild_Tenant).Position.Y;
            if X = -1 or Y = -1 then
               Time_Stamp := To_Duration(Clock - Start_Time);
               Store_Trace;  
               Alive := False;
               Players(Wild_Tenant).Symbol := '#';
            else 
               select
                  Board(X, Y).Lock;
                  if X = Players(Wild_Tenant).Position.X and Y = Players(Wild_Tenant).Position.Y then
                     Players(Wild_Tenant).Position.X := -1;
                     Players(Wild_Tenant).Position.Y := -1;
                     Time_Stamp := To_Duration(Clock - Start_Time);
                     Board(X, Y).Clear;
                     Store_Trace;  
                     Alive := False;
                     Board(X, Y).Unlock;
                  else
                     -- Moved during dying
                     Board(X, Y).Unlock;
                  end if;
               or 
                  delay Max_Delay/10.0;
               end select;
            end if;
            exit when not Alive;
         end loop;
      end Safe_Disappear;

  begin
    accept Init(Id: Integer; Seed: Integer) do
      Reset(G, Seed);
      Wild_Tenant := Id;
      Players(Wild_Tenant).Id := Id;
      Players(Wild_Tenant).Wild := True;
      Alive := False;
      Active_Wild_Tenants.Increment;
    end Init;

    accept Start do
      null;
    end Start;

    loop
      if not Alive then
        Safe_Appear;

      else
        -- Check if lifetime expired
        if Clock - Birth_Time > To_Time_Span(Wild_Tenant_Lifetime) then
          Safe_Disappear;
        end if;
      end if;
      
      -- Terminate if all travelers are done (simplified condition)
      exit when Zadanie3.Active_Travelers.Get_Count = 0;
    end loop;
    
    Active_Wild_Tenants.Decrement;
    Printer.Report(Traces);
  end Wild_Tenant_Task_Type;

  -- local for main task
  Travel_Tasks: array(0 .. Nr_Of_Travelers-1) of Traveler_Task_Type;
  Wild_Tenant_Tasks: array(0 .. Nr_Of_Wild_Tenants-1) of Wild_Tenant_Task_Type;
  Symbol : Character := 'A';

begin
  -- Print the line with the parameters needed for display script:
  Put_Line(
      "-1 " &
      Integer'Image(Nr_Of_Travelers + Nr_Of_Wild_Tenants + Traps_Count) & " " &
      Integer'Image(Board_Width) & " " &
      Integer'Image(Board_Height)
    );

   -- set up traps
   declare
   Traps_done : Integer := 0;
   X, Y : Integer;
   G : Generator;
   begin
      reset(G);
      loop
         X := Integer(Float'Floor(Ada.Numerics.Float_Random.Random(G) * Float(Board_Width)));
         Y := Integer(Float'Floor(Ada.Numerics.Float_Random.Random(G) * Float(Board_Height)));

         if not Board(X, Y).Is_Trapped then
            Board(X, Y).Set_Trap(Nr_Of_Travelers + Nr_Of_Wild_Tenants + Traps_done, X, Y);
            Traps_done := Traps_done + 1;
         end if;
      exit when Traps_done >= Traps_Count;
      end loop;
   end;

  -- init travelers tasks
  for I in Travel_Tasks'Range loop
    Travel_Tasks(I).Init(I, Seeds(I+1), Symbol);
    Symbol := Character'Succ(Symbol);
  end loop;
  -- init wild tenant tasks
  for I in Wild_Tenant_Tasks'Range loop
    Wild_Tenant_Tasks(I).Init(Nr_Of_Travelers + I, Wild_Tenant_Seeds(I+1));
  end loop;

  -- start all tasks
  for I in Travel_Tasks'Range loop
    Travel_Tasks(I).Start;
  end loop;
  for I in Wild_Tenant_Tasks'Range loop
    Wild_Tenant_Tasks(I).Start;
  end loop;
  
  loop
   exit when Active_Wild_Tenants.Get_Count = 0;
   delay 0.05; -- Small delay
  end loop;

  declare
  Local_Traces : Traces_Sequence_Type;
  begin
    for X in 0 .. Board_Width - 1 loop
      for Y in 0 .. Board_Height - 1 loop
        Board(X, Y).Export_Traces(Local_Traces);
        Printer.Report(Local_Traces);
      end loop;
    end loop;
  end;
end Zadanie3;
