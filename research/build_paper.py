"""Build an IEEE-style, two-column PDF and an editable IEEEtran LaTeX source.

Usage: python3 research/build_paper.py
Requires reportlab. All reported measurements come from the saved experiment.
"""
from pathlib import Path
import html
import json
import math
import re
import statistics

from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_JUSTIFY, TA_LEFT
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.utils import simpleSplit
from reportlab.pdfgen import canvas
from reportlab.graphics.shapes import Drawing, Rect, Line, String, Polygon, Circle
from reportlab.platypus import BaseDocTemplate, PageTemplate, Frame, Paragraph, Table, TableStyle, Spacer, PageBreak, KeepTogether

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "output/research"
OUT.mkdir(parents=True, exist_ok=True)
PDF_OUT = ROOT / "output/pdf/tunnelforge_ieee_research_paper.pdf"
PDF_OUT.parent.mkdir(parents=True, exist_ok=True)
DATA = json.loads((OUT / "data/results.json").read_text())
RAW = (ROOT / "research/manuscript.md").read_text()
W, H, M, GAP, COL = 612, 792, 45, 18, 252
BLACK, GRAY, LIGHT = colors.black, colors.HexColor("#444444"), colors.HexColor("#f0f0f0")

STYLE = {
 "body":ParagraphStyle("body",fontName="Times-Roman",fontSize=10,leading=11.5,alignment=TA_JUSTIFY,firstLineIndent=10,spaceAfter=3.2),
 "abstract":ParagraphStyle("abstract",fontName="Times-Bold",fontSize=9,leading=10.3,alignment=TA_JUSTIFY,spaceAfter=7),
 "section":ParagraphStyle("section",fontName="Times-Roman",fontSize=10,leading=12,alignment=TA_CENTER,spaceBefore=9,spaceAfter=5,keepWithNext=True),
 "sub":ParagraphStyle("sub",fontName="Times-Italic",fontSize=10,leading=11.5,spaceBefore=5,spaceAfter=3,keepWithNext=True),
 "caption":ParagraphStyle("caption",fontName="Times-Roman",fontSize=8,leading=9.2,spaceBefore=4,spaceAfter=7),
 "tablecap":ParagraphStyle("tablecap",fontName="Times-Roman",fontSize=8,leading=9.4,alignment=TA_CENTER,spaceBefore=6,spaceAfter=4),
 "cell":ParagraphStyle("cell",fontName="Times-Roman",fontSize=8,leading=9),
 "headcell":ParagraphStyle("headcell",fontName="Times-Bold",fontSize=8,leading=9),
 "ref":ParagraphStyle("ref",fontName="Times-Roman",fontSize=8,leading=9.2,leftIndent=13,firstLineIndent=-13,spaceAfter=4),
}

def fmt(s):
 s=s.replace("—","-").replace("–","-")
 s=html.escape(html.unescape(s), quote=False)
 s=re.sub(r"\*\*(.*?)\*\*",r"<b>\1</b>",s)
 # Links are clickable while still wrapping in narrow reference columns.
 if s.startswith("["):
  s=re.sub(r"https://[^\s]+",lambda m:f'<link href="{m.group(0)}" color="black">{m.group(0)}</link>',s)
 return s

def P(s,style="body"): return Paragraph(fmt(s),STYLE[style])

def arrow(d,x1,y1,x2,y2,dashed=False):
 line=Line(x1,y1,x2,y2,strokeColor=BLACK,strokeWidth=.65)
 if dashed: line.strokeDashArray=[3,2]
 d.add(line)
 angle=math.atan2(y2-y1,x2-x1)
 pts=[x2,y2]
 for a in [angle+2.7,angle-2.7]: pts.extend([x2+4*math.cos(a),y2+4*math.sin(a)])
 d.add(Polygon(pts,fillColor=BLACK,strokeColor=BLACK))

def architecture():
 d=Drawing(COL,145)
 boxes=[(5,99,66,29,"HTTP caller"),(94,99,66,29,"Relay"),(94,37,66,29,"CLI client"),(184,37,66,29,"Local app")]
 for x,y,w,h,label in boxes:
  d.add(Rect(x,y,w,h,fillColor=LIGHT,strokeColor=BLACK,strokeWidth=.7))
  d.add(String(x+w/2,y+11,label,fontName="Times-Bold",fontSize=9,textAnchor="middle"))
 arrow(d,71,116,94,116);arrow(d,94,107,71,107)
 arrow(d,113,99,113,66);arrow(d,142,66,142,99)
 arrow(d,160,55,184,55);arrow(d,184,46,160,46)
 d.add(String(5,134,"Public-side interface",fontName="Times-Italic",fontSize=8))
 d.add(String(152,79,"WebSocket",fontName="Times-Roman",fontSize=8))
 d.add(String(4,12,"Relay state: tunnel registry + pending request map",fontName="Times-Roman",fontSize=8.5))
 d.add(Line(4,28,248,28,strokeColor=GRAY,strokeWidth=.5))
 return [d,P("Fig. 1. Logical architecture. The client initiates the WebSocket connection; all experiment endpoints use loopback sockets.","caption")]

def sequence():
 d=Drawing(COL,147)
 xs=[22,92,163,232]
 for x,label in zip(xs,["Caller","Relay","CLI","App"]):
  d.add(String(x,137,label,fontName="Times-Bold",fontSize=8.5,textAnchor="middle"))
  d.add(Line(x,10,x,129,strokeColor=GRAY,strokeWidth=.4,strokeDashArray=[2,2]))
 arrows=[(0,1,119,"HTTP request"),(1,2,100,"http_request(id)"),(2,3,81,"local HTTP"),(3,2,62,"HTTP response"),(2,1,43,"http_response(id)"),(1,0,24,"HTTP response")]
 for a,b,y,label in arrows:
  arrow(d,xs[a],y,xs[b],y)
  d.add(String((xs[a]+xs[b])/2,y+4,label,fontName="Times-Roman",fontSize=7.3,textAnchor="middle"))
 return [d,P("Fig. 2. Successful forwarding sequence. A timeout or caller disconnect instead triggers cancel(id) from relay to client and cancellation of the local HTTP context.","caption")]

def table(title,headers,rows,widths=None):
 data=[[P(x,"headcell") for x in headers]]+[[P(str(x),"cell") for x in row] for row in rows]
 t=Table(data,colWidths=widths or [COL/len(headers)]*len(headers),hAlign="CENTER")
 t.setStyle(TableStyle([("VALIGN",(0,0),(-1,-1),"TOP"),("LINEABOVE",(0,0),(-1,0),.65,BLACK),("LINEBELOW",(0,0),(-1,0),.45,BLACK),("LINEBELOW",(0,-1),(-1,-1),.65,BLACK),("LEFTPADDING",(0,0),(-1,-1),3),("RIGHTPADDING",(0,0),(-1,-1),3),("TOPPADDING",(0,0),(-1,-1),3),("BOTTOMPADDING",(0,0),(-1,-1),3)]))
 return [KeepTogether([P(title,"tablecap"),t,Spacer(1,5)])]

def metrics(b,c,m):
 rs=[r for r in DATA["results"] if r["payload_bytes"]==b and r["concurrency"]==c and r["mode"]==m]
 return {"p50":statistics.median(r["median_ms"] for r in rs),"p95":statistics.median(r["p95_ms"] for r in rs),"rps":statistics.median(r["requests_per_second"] for r in rs),"min":min(r["requests_per_second"] for r in rs),"max":max(r["requests_per_second"] for r in rs)}

def performance():
 rows=[]
 for b in [1024,65536]:
  for c in [1,8,32]:
   for m in ["direct","tunnel"]:
    v=metrics(b,c,m)
    rows.append([f"{b//1024}",str(c),"D" if m=="direct" else "T",f'{v["p50"]:.3f}',f'{v["p95"]:.3f}',f'{v["rps"]:,.1f}'])
 return table("TABLE IV\nLOCAL PERFORMANCE: MEDIANS ACROSS THREE RUNS\nD = DIRECT; T = TUNNEL; LATENCY IN ms; RATE IN requests/s",["KiB","C","Mode","p50","p95","Rate"],rows,[27,20,32,47,47,79])

def perfplot():
 d=Drawing(COL,156)
 left,right,bottom,top=42,244,36,141
 def xp(v):return left+(math.log10(v)-2)/(5-2)*(right-left)
 for v in [100,1000,10000,100000]:
  x=xp(v);d.add(Line(x,bottom,x,top,strokeColor=colors.HexColor("#cccccc"),strokeWidth=.4));d.add(String(x,24,str(v),fontName="Times-Roman",fontSize=7,textAnchor="middle"))
 for i,(b,c) in enumerate([(1024,1),(1024,8),(1024,32),(65536,1),(65536,8),(65536,32)]):
  y=top-8-i*17
  d.add(String(left-5,y-2,f"{b//1024}K / {c}",fontName="Times-Roman",fontSize=7.4,textAnchor="end"))
  for mode,off,fill in [("direct",2,colors.white),("tunnel",-2,BLACK)]:
   v=metrics(b,c,mode); yy=y+off
   d.add(Line(xp(v["min"]),yy,xp(v["max"]),yy,strokeColor=BLACK,strokeWidth=1))
   d.add(Circle(xp(v["rps"]),yy,2.5,strokeColor=BLACK,fillColor=fill))
 d.add(String(143,10,"Throughput (requests/s, logarithmic scale)",fontName="Times-Roman",fontSize=8,textAnchor="middle"))
 d.add(String(44,151,"Payload / C",fontName="Times-Italic",fontSize=7.5))
 return [d,P("Fig. 3. Median run throughput: open circles indicate direct HTTP and filled circles indicate the tunnel. Horizontal lines show observed min-max ranges across three runs, not confidence intervals.","caption")]

MARKERS={
 "ARCHITECTURE":architecture,
 "SEQUENCE":sequence,
 "PROTOCOL_TABLE":lambda:table("TABLE I\nAPPLICATION PROTOCOL MESSAGES",["Type","Scope and purpose"],[
 ("register","Connection: submit token"),("registered","Connection: return tunnel ID and URL"),("http_request","Request: method, path, query, headers, body"),("http_response","Request: status, headers, body"),("cancel","Request: cancel local HTTP context"),("heartbeat","Connection: application liveness message"),("error","Connection: registration/protocol failure")],[77,175]),
 "BASE64_EQUATION":lambda:[P("L(B) = 4 ceil(B / 3) &nbsp;&nbsp;&nbsp;&nbsp; (1)","tablecap")],
 "LIMITS_TABLE":lambda:table("TABLE II\nIMPLEMENTED RESOURCE AND TIMING LIMITS",["Parameter","Value"],[("HTTP body / WebSocket message","1 MiB / 2 MiB"),("Active local requests per connection","64"),("Relay pending requests / outgoing queue","1,000 / 128"),("Relay / local HTTP timeout","30 s / 35 s"),("Application heartbeat / relay ping","20 s / 25 s"),("Socket read deadline / backoff cap","60 s / 30 s")],[175,77]),
 "FUNCTIONAL_TABLE":lambda:table("TABLE III\nOBSERVED FUNCTIONAL AND FAILURE OUTCOMES",["Scenario","Observed result"],[("Method, path, query, headers, body","Preserved in round trip"),("20 concurrent requests","Correct response matching"),("Binary, redirect, repeated cookies","Exact bytes; 302; two cookies"),("Connection-nominated headers","Removed in both directions"),("Oversize request / response","413 / 502"),("Stopped local application","502"),("Incorrect shared token","Rejected without retries"),("150 ms relay timeout","504; local context cancelled"),("Dropped WebSocket connection","Second registration observed"),("Wrong response owner (unit test)","Rejected; pending entry retained")],[143,109]),
 "PERFORMANCE_TABLE":performance,
 "PERFORMANCE_FIGURE":perfplot,
}

def first(canvas,doc):
 canvas.saveState()
 canvas.setFont("Times-Roman",21)
 canvas.drawCentredString(W/2,739,"TunnelForge: Design and Experimental Evaluation")
 canvas.drawCentredString(W/2,714,"of a Go-Based Reverse HTTP Tunnel over WebSocket")
 authors=[("Soham Agarwal","202452332"),("Prem Sharma","202452328"),("Aghnivesh Thottumkara","202451175")]
 for x,(name,roll) in zip([132,306,480],authors):
  canvas.setFont("Times-Roman",11);canvas.drawCentredString(x,680,name)
  canvas.setFont("Times-Roman",9);canvas.drawCentredString(x,666,f"Roll No. {roll}")
 canvas.setFont("Times-Italic",10)
 canvas.drawCentredString(W/2,647,"Department of Information Technology")
 canvas.drawCentredString(W/2,634,"Indian Institute of Information Technology")
 canvas.setFont("Times-Roman",9.5)
 canvas.drawCentredString(W/2,619,"Under the supervision of Dr. Rishabh Gupta")
 canvas.restoreState()
 footer(canvas,doc)

def footer(canvas,doc):
 canvas.saveState();canvas.setFont("Times-Roman",8);canvas.drawCentredString(W/2,30,str(doc.page));canvas.restoreState()

def build_pdf():
 doc=BaseDocTemplate(str(PDF_OUT),pagesize=(W,H),leftMargin=M,rightMargin=M,topMargin=54,bottomMargin=54,title="TunnelForge: Design and Experimental Evaluation of a Go-Based Reverse HTTP Tunnel over WebSocket",author="Soham Agarwal; Prem Sharma; Aghnivesh Thottumkara")
 def frames(top):
  return [Frame(M,54,COL,top-54,id="left",leftPadding=0,rightPadding=0,topPadding=0,bottomPadding=0),Frame(M+COL+GAP,54,COL,top-54,id="right",leftPadding=0,rightPadding=0,topPadding=0,bottomPadding=0)]
 doc.addPageTemplates([PageTemplate(id="first",frames=frames(600),onPage=first,autoNextPageTemplate="normal"),PageTemplate(id="normal",frames=frames(738),onPage=footer)])
 pages=RAW.split("<!-- PAGE -->")
 story=[]
 for i,page in enumerate(pages):
  if i==0: page=page[page.index("**Abstract"):]
  for para in re.split(r"\n\s*\n",page.strip()):
   para=para.strip()
   if not para:continue
   marker=re.fullmatch(r"\[\[(\w+)\]\]",para)
   if marker:
    story.extend(MARKERS[marker.group(1)]());continue
   if para.startswith("### "):story.append(P(para[4:],"sub"))
   elif para.startswith("## "):story.append(P(para[3:],"section"))
   elif para.startswith("**Abstract") or para.startswith("**Index Terms"):story.append(P(para,"abstract"))
   elif re.match(r"\[\d+\]",para):story.append(P(para,"ref"))
   else:story.append(P(para))
  if i<len(pages)-1:story.append(PageBreak())
 doc.build(story)

def tex_escape(s):
 s=html.unescape(s).replace("—","---").replace("–","--")
 s=s.replace("\\",r"\textbackslash{}")
 for a,b in [("&",r"\&"),("%",r"\%"),("_",r"\_"),("#",r"\#"),("<",r"$<$"),(">",r"$>$")]:s=s.replace(a,b)
 s=s.replace("“","``").replace("”","''")
 return s

def build_tex():
 # IEEEtran remains editable and can be compiled in Overleaf or a TeX installation.
 lines=[r"\documentclass[conference]{IEEEtran}",r"\usepackage[T1]{fontenc}",r"\usepackage{amsmath,graphicx,booktabs,url}",r"\usepackage[hidelinks]{hyperref}",r"\begin{document}",r"\title{TunnelForge: Design and Experimental Evaluation of a Go-Based Reverse HTTP Tunnel over WebSocket}",r"\author{\IEEEauthorblockN{Soham Agarwal (202452332), Prem Sharma (202452328),\\ Aghnivesh Thottumkara (202451175)}\IEEEauthorblockA{Department of Information Technology\\Indian Institute of Information Technology\\Under the supervision of Dr. Rishabh Gupta}}",r"\maketitle"]
 text=RAW[RAW.index("**Abstract"):]
 for para in re.split(r"\n\s*\n",text.strip()):
  para=para.strip()
  if para=="<!-- PAGE -->":continue
  marker=re.fullmatch(r"\[\[(\w+)\]\]",para)
  if marker:
   key=marker.group(1)
   if key=="BASE64_EQUATION":lines.append(r"\begin{equation}L(B)=4\left\lceil B/3\right\rceil\end{equation}")
   elif key in ["ARCHITECTURE","SEQUENCE","PERFORMANCE_FIGURE"]:
    caption={"ARCHITECTURE":"Logical architecture and relay state.","SEQUENCE":"Forwarding sequence and cancellation branch.","PERFORMANCE_FIGURE":"Run throughput; open circles: direct; filled circles: tunnel. Lines: observed min--max ranges, not confidence intervals."}[key]
    lines.extend([r"\begin{figure}[t]\centering",r"\includegraphics[width=\columnwidth]{figures/"+key.lower()+".pdf}",r"\caption{"+caption+"}",r"\end{figure}"])
   else:
    # Render the same vector tables into standalone PDFs to preserve all values.
    lines.extend([r"\begin{figure}[t]\centering",r"\includegraphics[width=\columnwidth]{figures/"+key.lower()+".pdf}",r"\end{figure}"])
   continue
  if para.startswith("**Abstract"):
   body=para.split("**",2)[2].lstrip("—- ");lines.extend([r"\begin{abstract}",tex_escape(body),r"\end{abstract}"])
  elif para.startswith("**Index Terms"):
   body=para.split("**",2)[2].lstrip("—- ");lines.extend([r"\begin{IEEEkeywords}",tex_escape(body),r"\end{IEEEkeywords}"])
  elif para.startswith("### "):
   lines.append(r"\subsection{"+tex_escape(re.sub(r"^[A-Z]\. ","",para[4:]))+"}")
  elif para.startswith("## "):
   title=re.sub(r"^[IVX]+\. ","",para[3:])
   lines.append(r"\section*{References}" if title=="REFERENCES" else r"\section{"+tex_escape(title.title())+"}")
  else:lines.append(tex_escape(para))
  lines.append("")
 lines.append(r"\end{document}")
 (OUT/"tunnelforge_ieee.tex").write_text("\n".join(lines))
 (OUT/"manuscript.md").write_text(RAW)
 figs=OUT/"figures";figs.mkdir(exist_ok=True)
 for key,factory in MARKERS.items():
  if key=="BASE64_EQUATION":continue
  flowables=factory()
  if key in ["ARCHITECTURE","SEQUENCE","PERFORMANCE_FIGURE"]:
   flowables=flowables[:1]
  flowables=[inner for f in flowables for inner in (f._content if isinstance(f,KeepTogether) else [f])]
  heights=[f.wrap(COL,1000)[1] for f in flowables]
  c=canvas.Canvas(str(figs/(key.lower()+".pdf")),pagesize=(COL,sum(heights)+2))
  y=sum(heights)+1
  for f,h in zip(flowables,heights):y-=h;f.drawOn(c,0,y)
  c.save()

if __name__=="__main__":
 build_pdf()
 build_tex()
 print(PDF_OUT)
