import json,re,sys
s=json.load(open(__import__('os').path.join(__import__('os').path.dirname(__import__('os').path.abspath(__file__)),'..','spec','openapi.json')))
C=s['components']['schemas']
INIT={'ui':'UI','id':'ID','url':'URL','uri':'URI','pan':'PAN','gstin':'GSTIN','ifsc':'IFSC','cin':'CIN','din':'DIN','micr':'MICR','utr':'UTR','nbin':'NBIN','dob':'DOB','dl':'DL','api':'API','sro':'SRO','udyam':'Udyam','rc':'RC','sms':'SMS','dlnumber':'DLNumber'}
def goname(n):
    toks=re.findall(r'[A-Z]+(?![a-z])|[A-Z]?[a-z]+|[0-9]+',n)
    return ''.join(INIT.get(t.lower(),t[:1].upper()+t[1:] if not t.isupper() else t) for t in toks)
RES={'PanVerificationResult':'PANVerification','GstinVerificationResult':'GSTINVerification','VehicleRcVerificationResult':'RCVerification','VoterIdVerificationResult':'VoterIDVerification','DrivingLicenseVerificationResult':'DrivingLicenseVerification','PassportVerificationResult':'PassportVerification','CinDirector':'CINDirector','CinVerificationResult':'CINVerification','UdyamVerificationResult':'UdyamVerification','PanToGstinEntry':'PANToGSTINEntry','PanToGstinResult':'PANToGSTIN','PanToUdyamResult':'PANToUdyam','IfscDetails':'IFSCDetails','BankAccountVerificationResult':'BankAccountVerification','Service':'Service','OrderReference':'OrderReference','OrderDetail':'OrderDetail','OrderStatusResponse':'OrderStatus','OrderPaymentResponse':'OrderPayment','OrderDocumentsResponse':'OrderDocuments','DocumentDownloadUrl':'DocumentDownloadURL','PaymentIntent':'PaymentIntent','UsageSummary':'UsageSummary','LogEntry':'LogEntry','LogsPage':'LogsPage'}
REQ={'PanVerifyRequest':'PANVerifyParams','GstinVerifyRequest':'GSTINVerifyParams','VehicleRcVerifyRequest':'RCVerifyParams','VoterIdVerifyRequest':'VoterIDVerifyParams','DrivingLicenseVerifyRequest':'DrivingLicenseVerifyParams','PassportVerifyRequest':'PassportVerifyParams','CinVerifyRequest':'CINVerifyParams','UdyamVerifyRequest':'UdyamVerifyParams','PanToGstinRequest':'PANToGSTINParams','PanToUdyamRequest':'PANToUdyamParams','BankAccountVerifyRequest':'BankAccountVerifyParams','EstampDutyCalculateRequest':'EstampDutyParams','EstampPartiesRequest':'EstampPartiesParams','CreateOrderRequest':'CreateOrderParams','CreatePaymentIntentRequest':'CreatePaymentIntentParams'}
INLINE={('UsageSummary','currentPeriod'):'UsagePeriod',('UsageSummary','callsByPeriod'):'UsageMonth',('UsageSummary','callsByDay'):'UsageDay'}
extra_types=[]
def gotype(parent,fname,sc,required,isreq):
    nullable=isinstance(sc.get('type'),list) and 'null' in sc['type']
    t=sc.get('type')
    if isinstance(t,list): t=[x for x in t if x!='null'][0]
    ptr = (not required) or nullable
    if (parent,fname)==('Service','uiSchema'): return 'json.RawMessage',False
    if '$ref' in sc:
        n=sc['$ref'].split('/')[-1]
        return RES[n],False
    if t is None: return 'json.RawMessage',False
    if t=='string': return 'string',(nullable)
    if t=='boolean': return 'bool',(ptr and not isreq) or nullable
    if t=='integer': return 'int64',(ptr and not isreq) or nullable
    if t=='number': return 'float64',(ptr and not isreq) or nullable
    if t=='array':
        it=sc.get('items',{})
        if (parent,fname) in INLINE:
            nm=INLINE[(parent,fname)]; extra_types.append((nm,it)); return '[]'+nm,False
        it_t,_=gotype(parent,fname+'Item',it,True,isreq); return '[]'+it_t,False
    if t=='object':
        if (parent,fname) in INLINE:
            nm=INLINE[(parent,fname)]; extra_types.append((nm,sc)); return nm,False
        ap=sc.get('additionalProperties')
        if isinstance(ap,dict) and ap.get('type')=='integer': return 'map[string]int64',False
        return 'map[string]json.RawMessage',False
    raise Exception((parent,fname,sc))
def comment(sc):
    d=(sc.get('description') or '').strip().replace('\n',' ')
    return d
def emit_struct(name,sc,isreq,out,doc=None):
    req=set(sc.get('required',[]))
    out.append(f"// {name}: {doc or comment(sc) or 'an API object.'}".rstrip())
    out.append(f"type {name} struct {{")
    for k,v in sc.get('properties',{}).items():
        gt,ptr=gotype(name,k,v,k in req,isreq)
        required=k in req
        opt=',omitempty' if (not required and (isreq or gt in('string','json.RawMessage') or gt.startswith('[]') or gt.startswith('map'))) else ''
        if isreq and gt=='map[string]json.RawMessage': gt='map[string]any'
        if ptr and gt!='json.RawMessage': gt='*'+gt
        if gt.startswith('*') or (not required and not isreq):
            opt=',omitempty'
        if required and not isreq and gt=='json.RawMessage': opt=''
        d=comment(v)
        if d: out.append(f"\t// {d}")
        out.append(f"\t{goname(k)} {gt} `json:\"{k}{opt}\"`")
    if not isreq:
        out.append("\n\t// Extra holds response fields this SDK version does not model, so nothing the API sends is lost.")
        out.append("\tExtra map[string]json.RawMessage `json:\"-\"`")
    out.append("}\n")
    if not isreq:
        out.append(f"// UnmarshalJSON decodes known fields and keeps the rest in Extra.\nfunc (m *{name}) UnmarshalJSON(b []byte) error {{\n\ttype plain {name}\n\treturn decodeWithExtra(b, (*plain)(m), &m.Extra)\n}}\n")
        out.append(f"// MarshalJSON encodes known fields and merges Extra back in.\nfunc (m {name}) MarshalJSON() ([]byte, error) {{\n\ttype plain {name}\n\treturn encodeWithExtra(plain(m), m.Extra)\n}}\n")
resp=['// Code generated by scripts/gen_models.py from spec/openapi.json. DO NOT EDIT.\n','package stampmitra\n','import "encoding/json"\n']
req=['// Code generated by scripts/gen_models.py from spec/openapi.json. DO NOT EDIT.\n','package stampmitra\n']
for k,v in C.items():
    if k in RES and v.get('type')=='object': emit_struct(RES[k],v,False,resp)
for nm,sc in extra_types:
    emit_struct(nm,sc,False,resp)
for k,v in C.items():
    if k in REQ: emit_struct(REQ[k],v,True,req)
open(__import__('os').path.join(__import__('os').path.dirname(__import__('os').path.abspath(__file__)),'..','models.go'),'w').write('\n'.join(resp))
open(__import__('os').path.join(__import__('os').path.dirname(__import__('os').path.abspath(__file__)),'..','params.go'),'w').write('\n'.join(req))
