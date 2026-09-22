create or replace procedure public.test_slow(in pi_data json, inout po_data json)
security definer
language plpgsql
as
$$
declare
  delay_ms int;
begin
  delay_ms := coalesce((pi_data->>'delayMs')::int, 0);

  if delay_ms > 0 then
    perform pg_sleep(delay_ms::float / 1000);
  end if;

  po_data := json_build_object('success', true, 'data', json_build_object('delayed', delay_ms));
end;
$$;
