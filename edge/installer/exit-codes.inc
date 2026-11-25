function ExitMessage(Code: Integer): String;
begin
  case Code of
    0: Result := 'Listo.';
    10: Result := 'Kuenta necesita Windows 10 u 11 de 64 bits.';
    11: Result := 'Ejecute el instalador como administrador del equipo.';
    12: Result := 'No hay espacio suficiente: se necesitan al menos 3 GB libres en el disco C:.';
    13: Result := 'Un puerto que Kuenta necesita ya está en uso por otro programa.';
    14: Result := 'Windows tiene bloqueada la ejecución de scripts por una directiva de la organización.';
    15: Result := 'No hay conexión con Kuenta. Revise el internet e intente de nuevo.';
    20: Result := 'El correo o la contraseña no son correctos.';
    21: Result := 'Solo un administrador del restaurante puede instalar la caja.';
    22: Result := 'La organización ya tiene una caja instalada.';
    23: Result := 'No se pudo registrar la caja en Kuenta.';
    30: Result := 'Este equipo tiene datos de una caja pero no su configuración. Contacte a soporte; no se cambió nada.';
    31: Result := 'No se pudo escribir la configuración de la caja.';
    32: Result := 'No se pudieron instalar los servicios de la caja. Revise C:\laguna-edge\logs.';
    33: Result := 'La caja se instaló pero no responde. Revise C:\laguna-edge\logs.';
    34: Result := 'La caja funciona, pero aún no se ha conectado a Kuenta. El inicio de sesión funcionará cuando tenga conexión.';
  else Result := 'Error inesperado (código ' + IntToStr(Code) + ').';
  end;
end;
