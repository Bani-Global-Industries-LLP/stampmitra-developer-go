import json,re,sys
s=json.load(open(__import__('os').path.join(__import__('os').path.dirname(__import__('os').path.abspath(__file__)),'..','spec','openapi.json')))
C=s['components']['schemas']
INIT={'ui':'UI','id':'ID','url':'URL','uri':'URI','pan':'PAN','gstin':'GSTIN','ifsc':'IFSC','cin':'CIN','din':'DIN','micr':'MICR','utr':'UTR','nbin':'NBIN','dob':'DOB','dl':'DL','api':'API','sro':'SRO','udyam':'Udyam','rc':'RC','sms':'SMS','dlnumber':'DLNumber','gstin':'GSTIN'}
def goname(n):
    toks=re.findall(r'[A-Z]+(?![a-z])|[A-Z]?[a-z]+|[0-9]+',n)
    return ''.join(INIT.get(t.lower(),t[:1].upper()+t[1:] if not t.isupper() else t) for t in toks)
RES={'PanVerificationResult':'PANVerification','GstinVerificationResult':'GSTINVerification','VehicleRcVerificationResult':'RCVerification','VoterIdVerificationResult':'VoterIDVerification','DrivingLicenseVerificationResult':'DrivingLicenseVerification','PassportVerificationResult':'PassportVerification','CinDirector':'CINDirector','CinVerificationResult':'CINVerification','UdyamVerificationResult':'UdyamVerification','PanToGstinEntry':'PANToGSTINEntry','PanToGstinResult':'PANToGSTIN','PanToUdyamResult':'PANToUdyam','IfscDetails':'IFSCDetails','BankAccountVerificationResult':'BankAccountVerification','Service':'Service','OrderReference':'OrderReference','OrderDetail':'OrderDetail','OrderStatusResponse':'OrderStatus','OrderPaymentResponse':'OrderPayment','OrderDocumentsResponse':'OrderDocuments','DocumentDownloadUrl':'DocumentDownloadURL','PaymentIntent':'PaymentIntent','UsageSummary':'UsageSummary','LogEntry':'LogEntry','LogsPage':'LogsPage',
'GstinSplitAddress':'GSTINSplitAddress','GstinAdditionalAddress':'GSTINAdditionalAddress','VoterIdSplitAddress':'VoterIDSplitAddress','DrivingLicenseSplitAddress':'DrivingLicenseSplitAddress','DrivingLicenseAddressEntry':'DrivingLicenseAddressEntry','DrivingLicenseBadgeDetail':'DrivingLicenseBadgeDetail','DrivingLicenseValidityPeriod':'DrivingLicenseValidityPeriod','DrivingLicenseValidity':'DrivingLicenseValidity','DrivingLicenseDetails':'DrivingLicenseDetails','CinDirectorDetail':'CINDirectorDetail','UdyamClassificationHistoryEntry':'UdyamClassificationHistoryEntry','UdyamSplitAddress':'UdyamSplitAddress','UdyamUnitLocation':'UdyamUnitLocation','UdyamNicCode':'UdyamNICCode',
'AccountBalance':'AccountBalance','AccountUsage':'AccountUsage','EstampStateAvailability':'EstampStateAvailability','EstampStatesAvailability':'EstampStatesAvailability','EstampState':'EstampState','EstampDistrict':'EstampDistrict','EstampSro':'EstampSRO','EstampRequirements':'EstampRequirements','EstampOrderQuote':'EstampOrderQuote','EstampValidationIssue':'EstampValidationIssue','EstampDocumentRequirement':'EstampDocumentRequirement','EstampOrderCounts':'EstampOrderCounts','EstampValidateResult':'EstampValidateResult','EstampOrderStampRef':'EstampOrderStampRef','EstampOrderAdvisory':'EstampOrderAdvisory','EstampOrder':'EstampOrder','EstampOrderList':'EstampOrderList','EstampOrderExport':'EstampOrderExport','EstampPanCardStatus':'EstampPANCardStatus','EstampStamp':'EstampStamp','EstampStampList':'EstampStampList','EstampPanCardResult':'EstampPANCardResult','EstampDocument':'EstampDocument','EstampCertificate':'EstampCertificate','EstampWalletPart':'EstampWalletPart','EstampWallet':'EstampWallet','WalletBalance':'WalletBalance','StampDutyWalletTopup':'StampDutyWalletTopup','StampDutyWalletInvoice':'StampDutyWalletInvoice','StampDutyWalletInvoicesPage':'StampDutyWalletInvoicesPage','ServiceFeeWalletTopup':'ServiceFeeWalletTopup','WalletTransaction':'WalletTransaction','WalletTransactionsPage':'WalletTransactionsPage','WalletInvoice':'WalletInvoice','WalletInvoicesPage':'WalletInvoicesPage'}
NESTED_REQ={'EstampParty':'EstampParty','EstampPropertyArea':'EstampPropertyArea','EstampStampLine':'EstampStampLine'}
REQ={'PanVerifyRequest':'PANVerifyParams','GstinVerifyRequest':'GSTINVerifyParams','VehicleRcVerifyRequest':'RCVerifyParams','VoterIdVerifyRequest':'VoterIDVerifyParams','DrivingLicenseVerifyRequest':'DrivingLicenseVerifyParams','PassportVerifyRequest':'PassportVerifyParams','CinVerifyRequest':'CINVerifyParams','UdyamVerifyRequest':'UdyamVerifyParams','PanToGstinRequest':'PANToGSTINParams','PanToUdyamRequest':'PANToUdyamParams','BankAccountVerifyRequest':'BankAccountVerifyParams','EstampDutyCalculateRequest':'EstampDutyParams','EstampPartiesRequest':'EstampPartiesParams','CreateOrderRequest':'CreateOrderParams','CreatePaymentIntentRequest':'CreatePaymentIntentParams','EstampOrderRequest':'EstampOrderParams','EstampPanCardRequest':'EstampPANCardParams','WalletTopupRequest':'WalletTopupParams'}
# Inline objects get a name; identical shapes (ignoring descriptions/enums) share one type.
INLINE={('UsageSummary','currentPeriod'):'UsagePeriod',('UsageSummary','callsByPeriod'):'UsageMonth',('UsageSummary','callsByDay'):'UsageDay',
('AccountBalance','balance'):'Money',('AccountBalance','available'):'Money',('AccountBalance','lowBalanceThreshold'):'Money',
('AccountBalance','estimatedCallsRemaining'):'AccountEstimatedCalls',('AccountBalance','lastTopUp'):'AccountLastTopUp',('AccountBalance','signupCredit'):'AccountSignupCredit',
('AccountEstimatedCalls','cheapestProduct'):'AccountCheapestProduct',('AccountEstimatedCalls','sevenDayAverageCost'):'Money',('AccountCheapestProduct','price'):'Money',
('AccountLastTopUp','amount'):'Money',('AccountSignupCredit','granted'):'Money',('AccountSignupCredit','left'):'Money',
('AccountUsage','currentMonth'):'AccountUsagePeriod',('AccountUsage','previousMonth'):'AccountUsagePeriod',('AccountUsage','today'):'AccountUsageToday',('AccountUsage','limits'):'AccountUsageLimits',
('AccountUsagePeriod','spend'):'Money',('AccountUsagePeriod','byProduct'):'AccountUsageProduct',('AccountUsagePeriod','daily'):'AccountUsageDay',
('AccountUsageProduct','spend'):'Money',('AccountUsageDay','spend'):'Money',('AccountUsageToday','spend'):'Money',('AccountUsageToday','requestsPerDay'):'AccountUsageQuota',
('AccountUsageLimits','requestsPerMinute'):'AccountUsageLimit',('AccountUsageLimits','requestsPerDay'):'AccountUsageLimit',
('EstampRequirements','state'):'EstampRequirementsState',('EstampRequirements','article'):'EstampRequirementsArticle',('EstampRequirements','denomination'):'EstampDenominationRules',
('EstampRequirements','fields'):'EstampRequirementField',('EstampRequirements','exampleBody'):'EstampRequirementsExample',
('EstampOrder','documents'):'EstampOrderDocuments',('EstampOrder','failure'):'EstampOrderFailure',('EstampOrder','sandbox'):'EstampOrderSandbox',
('EstampOrderList','pagination'):'EstampPagination'}
VALUE_COMPAT={('BankAccountVerification','ifscDetails')}  # kept as a value for compatibility with 0.1.0
GONAME={('GSTINVerification','GSTIN'):'GSTINLegacy'}
extra_types=[]
seen_inline={}
def shape(sc):
    return json.dumps({k:(v.get('type'),v.get('items',{}).get('type') if isinstance(v.get('items'),dict) else None) for k,v in sc.get('properties',{}).items()},sort_keys=True)
def isnull(sc):
    t=sc.get('type'); return isinstance(t,list) and 'null' in t
def gotype(parent,fname,sc,required,isreq):
    """returns (go type, is pointer)"""
    if 'oneOf' in sc:
        alts=[x for x in sc['oneOf'] if x.get('type')!='null']
        if len(alts)==1 and len(sc['oneOf'])==2:
            t,_=gotype(parent,fname,alts[0],True,isreq); return t,True
        if [x.get('type') for x in alts]==['string',None] and alts[1].get('$ref','').endswith('EstampParty'):
            return 'Party',(not required)
        raise Exception(('oneOf',parent,fname))
    nullable=isnull(sc)
    t=sc.get('type')
    if isinstance(t,list): t=[x for x in t if x!='null'][0]
    ptr=(not required) or nullable
    if (parent,fname)==('Service','uiSchema'): return 'json.RawMessage',False
    if '$ref' in sc:
        n=sc['$ref'].split('/')[-1]
        nm=RES.get(n) or NESTED_REQ[n]
        return nm,(ptr and (parent,fname) not in VALUE_COMPAT)
    if t is None: return 'json.RawMessage',False
    if t=='string': return 'string',(nullable)
    if t=='boolean': return 'bool',(ptr and not isreq) or nullable
    if t=='integer': return 'int64',(ptr and not isreq) or nullable
    if t=='number': return 'float64',(ptr and not isreq) or nullable
    if t=='array':
        it=sc.get('items',{})
        if (parent,fname) in INLINE:
            nm=inline(parent,fname,it); return '[]'+nm,False
        it_t,_=gotype(parent,fname+'Item',it,True,isreq); return '[]'+it_t,False
    if t=='object':
        if (parent,fname) in INLINE:
            nm=inline(parent,fname,sc)
            return nm,(ptr and (parent,fname) not in {('UsageSummary','currentPeriod')} and not (parent in('UsageSummary',) ))
        ap=sc.get('additionalProperties')
        if isinstance(ap,dict) and ap.get('type')=='integer': return 'map[string]int64',False
        return 'map[string]json.RawMessage',False
    raise Exception((parent,fname,sc))
def inline(parent,fname,sc):
    nm=INLINE[(parent,fname)]
    key=(nm,shape(sc))
    if nm in seen_inline:
        return nm
    seen_inline[nm]=sc
    extra_types.append((nm,sc))
    return nm
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
        if gt=='Party' and not required: gt='*Party'; opt=',omitempty'
        d=comment(v)
        if d: out.append(f"\t// {d}")
        out.append(f"\t{GONAME.get((name,k)) or goname(k)} {gt} `json:\"{k}{opt}\"`")
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
i=0
while i<len(extra_types):
    nm,sc=extra_types[i]; i+=1
    if nm=='EstampRequirementsExample':
        emit_struct(nm,sc,True,resp); continue
    emit_struct(nm,sc,False,resp)
for k,v in C.items():
    if k in REQ: emit_struct(REQ[k],v,True,req)
for k,v in C.items():
    if k in NESTED_REQ: emit_struct(NESTED_REQ[k],v,True,req)
base=__import__('os').path.join(__import__('os').path.dirname(__import__('os').path.abspath(__file__)),'..')
open(base+'/models.go','w').write('\n'.join(resp))
open(base+'/params.go','w').write('\n'.join(req))
