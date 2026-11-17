import { render, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { GamePage } from "./GamePage";
const { mutate, mutateAsync, callbacks } = vi.hoisted(() => ({ mutate: vi.fn(), mutateAsync: vi.fn(), callbacks: {} as { act?: (type: string) => unknown } }));
vi.mock("../features/game/useGame", () => ({ useGame: () => ({ game: {id:"game",version:3,fen:"4k4/9/9/9/9/9/9/9/9/4K4 w",moves:[],legalMoves:[],human:"red",difficulty:"standard",phase:"playing",outcome:"ongoing"},session:{data:{profiles:[],recent:[]}},query:{},online:true,command:{mutate,mutateAsync},create:{} }) }));
vi.mock("../features/account/AccountNav", () => ({ AccountNav: () => null }));
vi.mock("../features/game/useSound", () => ({useSound:()=>[false,vi.fn()]}));
vi.mock("../features/game/board/ChessBoard", () => ({ChessBoard:()=>null}));
vi.mock("../features/game/GameActions", () => ({GameActions:(props:{act:(type:string)=>unknown})=>{callbacks.act=props.act;return null}}));
beforeEach(()=>{vi.stubGlobal("matchMedia",vi.fn(()=>({matches:false,addListener:vi.fn(),removeListener:vi.fn(),addEventListener:vi.fn(),removeEventListener:vi.fn()})))});
afterEach(()=>{cleanup();vi.clearAllMocks();vi.unstubAllGlobals()});
it("returns resignation completion to its confirmation without changing move mutation handling",async()=>{
 let finish!:(value:unknown)=>void;const pending=new Promise(resolve=>{finish=resolve});mutateAsync.mockReturnValue(pending);
 render(<MemoryRouter><GamePage/></MemoryRouter>);
 const result=callbacks.act!("resign");expect(result).toBe(pending);expect(mutateAsync).toHaveBeenCalledWith({type:"resign",move:undefined});
 finish({});await result;callbacks.act!("undo");expect(mutate).toHaveBeenCalledWith({type:"undo",move:undefined});
});
