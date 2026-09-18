import type { bridge } from './models'
import { rpc } from './transport'
export function AbsolutePath(arg1: string): Promise<bridge.FlagResult> {
  return rpc('AbsolutePath', [arg1])
}
export function CloseMMDB(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('CloseMMDB', [arg1, arg2])
}
export function CopyFile(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('CopyFile', [arg1, arg2])
}
export function Download(
  arg1: string,
  arg2: string,
  arg3: string,
  arg4: Record<string, string>,
  arg5: string,
  arg6: bridge.RequestOptions,
): Promise<bridge.HTTPResult> {
  return rpc('Download', [arg1, arg2, arg3, arg4, arg5, arg6])
}
export function Exec(
  arg1: string,
  arg2: Array<string>,
  arg3: bridge.ExecOptions,
): Promise<bridge.FlagResult> {
  return rpc('Exec', [arg1, arg2, arg3])
}
export function ExecBackground(
  arg1: string,
  arg2: Array<string>,
  arg3: string,
  arg4: string,
  arg5: bridge.ExecOptions,
): Promise<bridge.FlagResult> {
  return rpc('ExecBackground', [arg1, arg2, arg3, arg4, arg5])
}
export function ExitApp(): Promise<void> {
  return rpc('ExitApp', [])
}
export function FileExists(arg1: string): Promise<bridge.FlagResult> {
  return rpc('FileExists', [arg1])
}
export function FileSHA256(arg1: string): Promise<bridge.FlagResult> {
  return rpc('FileSHA256', [arg1])
}
export function GetEnv(arg1: string): Promise<any> {
  return rpc('GetEnv', [arg1])
}
export function GetInterfaces(): Promise<bridge.FlagResult> {
  return rpc('GetInterfaces', [])
}
export function GetSystemProxy(): Promise<bridge.FlagResult> {
  return rpc('GetSystemProxy', [])
}
export function GetSystemProxyBypass(): Promise<bridge.FlagResult> {
  return rpc('GetSystemProxyBypass', [])
}
export function IsStartup(): Promise<boolean> {
  return rpc('IsStartup', [])
}
export function KillProcess(arg1: number, arg2: number): Promise<bridge.FlagResult> {
  return rpc('KillProcess', [arg1, arg2])
}
export function ListServer(): Promise<bridge.FlagResult> {
  return rpc('ListServer', [])
}
export function MakeDir(arg1: string): Promise<bridge.FlagResult> {
  return rpc('MakeDir', [arg1])
}
export function MoveFile(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('MoveFile', [arg1, arg2])
}
export function OpenDir(arg1: string): Promise<bridge.FlagResult> {
  return rpc('OpenDir', [arg1])
}
export function OpenMMDB(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('OpenMMDB', [arg1, arg2])
}
export function OpenURI(arg1: string): Promise<bridge.FlagResult> {
  return rpc('OpenURI', [arg1])
}
export function ProcessInfo(arg1: number): Promise<bridge.FlagResult> {
  return rpc('ProcessInfo', [arg1])
}
export function ProcessMemory(arg1: number): Promise<bridge.FlagResult> {
  return rpc('ProcessMemory', [arg1])
}
export function QueryMMDB(arg1: string, arg2: string, arg3: string): Promise<bridge.FlagResult> {
  return rpc('QueryMMDB', [arg1, arg2, arg3])
}
export function ReadDir(arg1: string): Promise<bridge.FlagResult> {
  return rpc('ReadDir', [arg1])
}
export function ReadFile(arg1: string, arg2: bridge.IOOptions): Promise<bridge.FlagResult> {
  return rpc('ReadFile', [arg1, arg2])
}
export function RemoveFile(arg1: string): Promise<bridge.FlagResult> {
  return rpc('RemoveFile', [arg1])
}
export function Requests(
  arg1: string,
  arg2: string,
  arg3: Record<string, string>,
  arg4: string,
  arg5: bridge.RequestOptions,
): Promise<bridge.HTTPResult> {
  return rpc('Requests', [arg1, arg2, arg3, arg4, arg5])
}
export function RestartApp(): Promise<bridge.FlagResult> {
  return rpc('RestartApp', [])
}
export function SetSystemDNS(arg1: string, arg2: Array<string>): Promise<bridge.FlagResult> {
  return rpc('SetSystemDNS', [arg1, arg2])
}
export function SetSystemProxy(
  arg1: boolean,
  arg2: string,
  arg3: string,
  arg4: string,
  arg5: Array<string>,
): Promise<bridge.FlagResult> {
  return rpc('SetSystemProxy', [arg1, arg2, arg3, arg4, arg5])
}
export function ShowMainWindow(): Promise<void> {
  return rpc('ShowMainWindow', [])
}
export function StartServer(
  arg1: string,
  arg2: string,
  arg3: bridge.ServerOptions,
): Promise<bridge.FlagResult> {
  return rpc('StartServer', [arg1, arg2, arg3])
}
export function StopServer(arg1: string): Promise<bridge.FlagResult> {
  return rpc('StopServer', [arg1])
}
export function TcpPing(arg1: string, arg2: bridge.NetOptions): Promise<bridge.FlagResult> {
  return rpc('TcpPing', [arg1, arg2])
}
export function TcpRequest(
  arg1: string,
  arg2: string,
  arg3: bridge.NetOptions,
): Promise<bridge.FlagResult> {
  return rpc('TcpRequest', [arg1, arg2, arg3])
}
export function UdpRequest(
  arg1: string,
  arg2: string,
  arg3: bridge.NetOptions,
): Promise<bridge.FlagResult> {
  return rpc('UdpRequest', [arg1, arg2, arg3])
}
export function UnzipGZFile(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('UnzipGZFile', [arg1, arg2])
}
export function UnzipTarGZFile(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('UnzipTarGZFile', [arg1, arg2])
}
export function UnzipZIPFile(arg1: string, arg2: string): Promise<bridge.FlagResult> {
  return rpc('UnzipZIPFile', [arg1, arg2])
}
export function UpdateTray(arg1: bridge.TrayContent): Promise<void> {
  return rpc('UpdateTray', [arg1])
}
export function UpdateTrayAndMenus(
  arg1: bridge.TrayContent,
  arg2: Array<bridge.MenuItem>,
): Promise<void> {
  return rpc('UpdateTrayAndMenus', [arg1, arg2])
}
export function UpdateTrayMenus(arg1: Array<bridge.MenuItem>): Promise<void> {
  return rpc('UpdateTrayMenus', [arg1])
}
export function Upload(
  arg1: string,
  arg2: string,
  arg3: string,
  arg4: Record<string, string>,
  arg5: string,
  arg6: bridge.RequestOptions,
): Promise<bridge.HTTPResult> {
  return rpc('Upload', [arg1, arg2, arg3, arg4, arg5, arg6])
}
export function WriteFile(
  arg1: string,
  arg2: string,
  arg3: bridge.IOOptions,
): Promise<bridge.FlagResult> {
  return rpc('WriteFile', [arg1, arg2, arg3])
}
