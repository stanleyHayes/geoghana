import { ImageResponse } from "next/og";

export const alt = "GhanaGeo — Ghana's open location infrastructure";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpenGraphImage() {
  return new ImageResponse(<div style={{width:"100%",height:"100%",display:"flex",position:"relative",padding:"76px",background:"#08271f",color:"#effaf6",fontFamily:"sans-serif"}}><div style={{position:"absolute",right:"-80px",top:"-110px",width:"560px",height:"560px",borderRadius:"48% 52% 61% 39%",border:"3px solid #48bd99",transform:"rotate(-12deg)",opacity:.7}}/><div style={{display:"flex",flexDirection:"column",justifyContent:"space-between",width:"100%"}}><div style={{display:"flex",alignItems:"center",gap:"18px",fontSize:"28px",fontWeight:700}}><span style={{display:"flex",width:"58px",height:"58px",alignItems:"center",justifyContent:"center",borderRadius:"16px",background:"#48bd99",color:"#061f19"}}>GG</span><span>GhanaGeo</span></div><div style={{display:"flex",flexDirection:"column",gap:"24px"}}><div style={{display:"flex",flexDirection:"column",fontSize:"72px",fontWeight:750,lineHeight:1.02,letterSpacing:"-3px"}}><span>Every place in Ghana,</span><span>finally in one place.</span></div><div style={{fontSize:"25px",color:"#b9d2c9"}}>Open · source-aware · free to query</div></div></div></div>, size);
}
