create or replace procedure public.test1(in pi_data json, inout po_data json)
security definer
language plpgsql
as
$$
DECLARE
  out_data json;	
begin

  out_data := json_build_object('property', 'value', 'input', pi_data);
  po_data:= json_build_object('success', true, 'data', out_data);
end;
$$;